package migrate

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/acomagu/trie/v2"
	"github.com/itsByte/gomarkov"
	"github.com/itsByte/tgmarkovgo/backend"
)

var (
	jsonPath       = flag.String("jsonpath", "", "JSON Chains Path, set to enable migration. WARNING: WILL DESTROY EXISTING DATABASE")
	forceMigration = flag.Bool("force-migration", false, "Force migration even if the database already exists")
)

const (
	OldStartToken = "^"
	OldEndToken   = "$"
)

// OldChainJSON defines the structure of the old v1 JSON format.
type OldChainJSON struct {
	cID      int64
	Order    int                       `json:"int"`
	SpoolMap map[string]int            `json:"spool_map"`
	FreqMat  map[string]map[string]int `json:"freq_mat"`
}

func MigrateChains() {
	time := time.Now().Format(time.RFC3339)
	logPath := filepath.Join(*backend.BaseDataPath, "migration_"+time+".log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()
	defer log.SetOutput(os.Stderr)

	mw := io.MultiWriter(logFile, os.Stderr)
	log.SetOutput(mw)

	dbPath := filepath.Join(*backend.BaseDataPath, "db")

	// Check if the database already exists and force migration is not enabled
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) && !*forceMigration {
		return
	}

	if *jsonPath == "" {
		*jsonPath = *backend.BaseDataPath
	}

	files, err := filepath.Glob(filepath.Join(*jsonPath, "*.json"))
	if err != nil || len(files) == 0 {
		log.Printf("\nNo files found in '%s': %v", *jsonPath, err)
		return
	}

	numWorkers := runtime.NumCPU()

	storage, err := gomarkov.NewPebbleStorage(dbPath)
	if err != nil {
		log.Fatalf("Could not create new Pebble storage at '%s': %v", dbPath, err)
	}
	defer storage.Close()

	// Read Data
	log.Println("Reading files...")
	var allOldChains []OldChainJSON
	vocabulary := make(map[string]struct{})

	for _, file := range files {
		data, _ := os.ReadFile(file)
		var oldChain OldChainJSON
		json.Unmarshal(data, &oldChain)
		oldChain.cID, err = strconv.ParseInt(filepath.Base(file)[:len(filepath.Base(file))-5], 10, 64)
		if err != nil {
			log.Printf("Could not parse file ID from '%s': %v", file, err)
			continue
		}
		allOldChains = append(allOldChains, oldChain)

		// Build the vocabulary from all "next state" tokens
		idToWord := invertMap(oldChain.SpoolMap)
		for _, toMap := range oldChain.FreqMat {
			for toIDStr := range toMap {
				vocabulary[idToWord[mustAtoi(toIDStr)]] = struct{}{}
			}
		}
	}

	// Add special boundary tokens
	vocabulary[gomarkov.StartToken] = struct{}{}
	vocabulary[gomarkov.EndToken] = struct{}{}

	log.Printf("Found %d true atomic words across %d files.", len(vocabulary), len(allOldChains))

	// Populate DB
	log.Println("Creating vocabulary in DB and building ID map...")
	vocabList := slices.Collect(maps.Keys(vocabulary))
	if err := storage.GetOrCreateMultiple(vocabList); err != nil {
		log.Fatalf("Could not create vocabulary in new database: %v", err)
	}

	vocabRunes := [][]rune{}
	newID := []uint32{}
	for _, word := range vocabList {
		id, err := storage.GetWordID(word)
		if err != nil {
			log.Fatalf("FATAL: Could not look up ID for word '%s' that should exist: %v", word, err)
		}
		newID = append(newID, id)
		vocabRunes = append(vocabRunes, []rune(word))
	}
	log.Println("Successfully created new vocabulary and ID map.")

	// Build Trie
	log.Println("Building vocabulary Trie...")
	trie := trie.New(vocabRunes, newID)

	log.Println("Dispatching workers...")
	jobs := make(chan OldChainJSON, len(allOldChains))
	results := make(chan []gomarkov.Transition, len(allOldChains))
	var wg sync.WaitGroup

	for range numWorkers {
		wg.Add(1)
		go transitionWorker(&wg, jobs, results, trie)
	}

	for _, oldChain := range allOldChains {
		jobs <- oldChain
	}
	close(jobs)

	wg.Wait()
	close(results)

	// Collect results
	var allTransitions []gomarkov.Transition
	for result := range results {
		allTransitions = append(allTransitions, result...)
	}
	log.Printf("Prepared %d total transitions (including lower orders for backoff).", len(allTransitions))

	log.Println("Executing bulk import...")
	if err := storage.AddTransitions(allTransitions); err != nil {
		log.Fatalf("FATAL: Failed during bulk import: %v", err)
	}

	log.Println("Migration completed successfully!")
}

// transitionWorker processes chains and sends the resulting transitions to the results channel
func transitionWorker(wg *sync.WaitGroup, chains <-chan OldChainJSON, results chan<- []gomarkov.Transition, trie trie.Tree[rune, uint32]) {
	defer wg.Done()
	for oldChain := range chains {
		var chainTransitions []gomarkov.Transition
		idToWord := invertMap(oldChain.SpoolMap)

		for fromIDStr, toMap := range oldChain.FreqMat {
			fromNgramKey := idToWord[mustAtoi(fromIDStr)]

			parsedNgram := ParseNgramKey(fromNgramKey, oldChain.Order, trie)
			if parsedNgram == nil {
				log.Printf("Could not parse key '%s'. Skipping.", fromNgramKey)
				continue
			}

			for toWord, frequency := range toMap {
				for o := oldChain.Order; o >= 1; o-- {
					if len(parsedNgram) < o {
						continue
					}
					subNgram := parsedNgram[len(parsedNgram)-o:]
					subNgramIDs := make([]uint32, len(subNgram))

					validNgram := true
					for i, word := range subNgram {
						tokenToLookUp := word
						if i == 0 && word == OldStartToken {
							tokenToLookUp = gomarkov.StartToken
						} else if i == len(subNgram)-1 && word == OldEndToken {
							tokenToLookUp = gomarkov.EndToken
						}

						id, ok := trie.Trace([]rune(tokenToLookUp)).Terminal()
						if !ok {
							validNgram = false
							break
						}
						subNgramIDs[i] = id
					}
					if !validNgram {
						continue
					}

					// Get the actual word from its old ID string, then find its new ID
					actualNextWord := idToWord[mustAtoi(toWord)]

					tokenToLookUp := actualNextWord
					if actualNextWord == OldEndToken {
						tokenToLookUp = gomarkov.EndToken
					}

					nextID, ok := trie.Trace([]rune(tokenToLookUp)).Terminal()
					if !ok {
						// This should not happen if vocabulary was built correctly
						log.Printf("Could not find new ID for word '%s' (from old ID %s). Skipping.", tokenToLookUp, toWord)
						continue
					}

					chainTransitions = append(chainTransitions, gomarkov.Transition{
						CID:        oldChain.cID,
						CurrentIDs: subNgramIDs,
						NextID:     nextID,
						Frequency:  uint32(frequency),
					})
				}
			}
		}
		results <- chainTransitions
	}
}

// invertMap returns a map with keys and values swapped
func invertMap(m map[string]int) map[int]string {
	idToWord := make(map[int]string, len(m))
	for k, v := range m {
		idToWord[v] = k
	}
	return idToWord
}
func mustAtoi(s string) int { i, _ := strconv.Atoi(s); return i }

// ParseNgramKey parses n-gram keys
func ParseNgramKey(key string, order int, trie trie.Tree[rune, uint32]) []string {
	// Longest-match parse
	if parsed := tryGreedyParse(key, order, trie); parsed != nil {
		return parsed
	}

	// Handle consecutive underscores
	return strings.Split(key, "_")
}

// tryGreedyParse attempts to parse the key by recursively finding the longest
// possible prefix of the key that exists as a word in the vocabulary
func tryGreedyParse(key string, order int, trie trie.Tree[rune, uint32]) []string {
	// Try whole word
	if order == 1 {
		_, ok := trie.Trace([]rune(key)).Terminal()
		if ok {
			return []string{key}
		}
		return nil
	}

	// Get longest match by recursively parsing
	var bestParse []string
	var currentPrefix strings.Builder
	traversalTrie := trie
	for _, r := range key {
		currentPrefix.WriteRune(r)
		if traversalTrie = traversalTrie.TraceOne(r); traversalTrie == nil {
			break
		}
		if _, ok := traversalTrie.Terminal(); ok {
			word := currentPrefix.String()
			// Check if the found word is followed by a separator
			if len(key) > len(word) && key[len(word)] == '_' {
				remainder := key[len(word)+1:]
				// Check for longer prefix
				if subParse := tryGreedyParse(remainder, order-1, trie); subParse != nil {
					bestParse = append([]string{word}, subParse...)
				}
			}
		}
	}

	// If no underscore-separated parse was found, check if the whole key is a single word
	if len(bestParse) == 0 {
		if _, ok := trie.Trace([]rune(key)).Terminal(); ok {
			return []string{key}
		}
	}

	if len(bestParse) > 0 {
		return bestParse
	}

	return nil
}
