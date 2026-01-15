// Command generate_demo creates a demo database with sample data from public domain books.
// Usage: go run cmd/generate_demo/main.go [-db path/to/demo.db]
package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/entities"
)

const defaultDemoDatabasePath = "./demo/demo.db"

func main() {
	dbPath := flag.String("db", defaultDemoDatabasePath, "path to the demo database file")
	flag.Parse()

	log.Printf("Generating demo database at %s...", *dbPath)

	// Delete existing demo database to start fresh
	if err := os.Remove(*dbPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("Failed to remove existing demo database: %v", err)
	}

	// Create database at demo path
	db, err := database.NewDatabase(*dbPath)
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create tags
	tags := createTags(db)

	// Create books with highlights (tags stored separately to avoid duplication)
	bookConfigs := getPublicDomainBooks()

	for _, cfg := range bookConfigs {
		if err := db.SaveBook(&cfg.Book); err != nil {
			log.Printf("Failed to save book %s: %v", cfg.Book.Title, err)
			continue
		}
		log.Printf("Saved: %s by %s (%d highlights)", cfg.Book.Title, cfg.Book.Author, len(cfg.Book.Highlights))

		// Add tags to the book using the proper API to avoid duplicates
		for _, tagName := range cfg.TagNames {
			if tag, ok := tags[tagName]; ok {
				if err := db.AddTagToBook(cfg.Book.ID, tag.ID); err != nil {
					log.Printf("Failed to add tag %s to book %s: %v", tagName, cfg.Book.Title, err)
				}
			}
		}
	}

	// Add vocabulary words
	addVocabularyWords(db)

	log.Println("Demo database generated successfully!")
}

func createTags(db *database.Database) map[string]entities.Tag {
	tagNames := []string{
		"philosophy",
		"fiction",
		"classic",
		"science",
	}

	tags := make(map[string]entities.Tag)
	for _, name := range tagNames {
		tag, err := db.CreateTag(name, 0) // userID 0 for demo
		if err != nil {
			log.Printf("Failed to create tag %s: %v", name, err)
			continue
		}
		tags[name] = *tag
	}
	return tags
}

// BookConfig holds a book and its tag names for deferred tag assignment.
type BookConfig struct {
	Book     entities.Book
	TagNames []string
}

func getPublicDomainBooks() []BookConfig {
	now := time.Now()

	return []BookConfig{
		// Marcus Aurelius - Meditations (Public Domain)
		{
			TagNames: []string{"philosophy", "classic"},
			Book: entities.Book{
				Title:           "Meditations",
				Author:          "Marcus Aurelius",
				Source:          entities.Source{Name: "demo", DisplayName: "Demo Import"},
				PublicationYear: 180,
				Highlights: []entities.Highlight{
					{
						Text:          "You have power over your mind - not outside events. Realize this, and you will find strength.",
						CreatedAt:     now,
						LocationValue: 1,
						IsFavorite:    true,
					},
					{
						Text:          "The happiness of your life depends upon the quality of your thoughts.",
						CreatedAt:     now,
						LocationValue: 2,
					},
					{
						Text:          "Waste no more time arguing about what a good man should be. Be one.",
						CreatedAt:     now,
						LocationValue: 3,
					},
					{
						Text:          "Very little is needed to make a happy life; it is all within yourself, in your way of thinking.",
						CreatedAt:     now,
						LocationValue: 4,
					},
					{
						Text:          "The soul becomes dyed with the color of its thoughts.",
						CreatedAt:     now,
						LocationValue: 5,
						IsFavorite:    true,
					},
					{
						Text:          "Accept the things to which fate binds you, and love the people with whom fate brings you together, and do so with all your heart.",
						CreatedAt:     now,
						LocationValue: 6,
					},
					{
						Text:          "When you arise in the morning, think of what a precious privilege it is to be alive - to breathe, to think, to enjoy, to love.",
						CreatedAt:     now,
						LocationValue: 7,
					},
					{
						Text:          "Never esteem anything as of advantage to you that will make you break your word or lose your self-respect.",
						CreatedAt:     now,
						LocationValue: 8,
					},
				},
			},
		},

		// Seneca - Letters from a Stoic (Public Domain)
		{
			TagNames: []string{"philosophy", "classic"},
			Book: entities.Book{
				Title:           "Letters from a Stoic",
				Author:          "Seneca",
				Source:          entities.Source{Name: "demo", DisplayName: "Demo Import"},
				PublicationYear: 65,
				Highlights: []entities.Highlight{
					{
						Text:          "We suffer more often in imagination than in reality.",
						CreatedAt:     now,
						LocationValue: 1,
						IsFavorite:    true,
					},
					{
						Text:          "Luck is what happens when preparation meets opportunity.",
						CreatedAt:     now,
						LocationValue: 2,
					},
					{
						Text:          "It is not that we have a short time to live, but that we waste a lot of it.",
						CreatedAt:     now,
						LocationValue: 3,
					},
					{
						Text:          "Difficulties strengthen the mind, as labor does the body.",
						CreatedAt:     now,
						LocationValue: 4,
					},
					{
						Text:          "True happiness is to enjoy the present, without anxious dependence upon the future.",
						CreatedAt:     now,
						LocationValue: 5,
					},
					{
						Text:          "Associate with people who are likely to improve you. Welcome those whom you are capable of improving.",
						CreatedAt:     now,
						LocationValue: 6,
					},
				},
			},
		},

		// Charles Darwin - On the Origin of Species (Public Domain)
		{
			TagNames: []string{"science", "classic"},
			Book: entities.Book{
				Title:           "On the Origin of Species",
				Author:          "Charles Darwin",
				Source:          entities.Source{Name: "demo", DisplayName: "Demo Import"},
				PublicationYear: 1859,
				Highlights: []entities.Highlight{
					{
						Text:          "It is not the strongest of the species that survives, nor the most intelligent that survives. It is the one that is most adaptable to change.",
						CreatedAt:     now,
						LocationValue: 1,
						IsFavorite:    true,
					},
					{
						Text:          "A man who dares to waste one hour of time has not discovered the value of life.",
						CreatedAt:     now,
						LocationValue: 2,
					},
					{
						Text:          "In the long history of humankind those who learned to collaborate and improvise most effectively have prevailed.",
						CreatedAt:     now,
						LocationValue: 3,
					},
					{
						Text:          "The love for all living creatures is the most noble attribute of man.",
						CreatedAt:     now,
						LocationValue: 4,
					},
					{
						Text:          "There is grandeur in this view of life, with its several powers, having been originally breathed into a few forms or into one.",
						CreatedAt:     now,
						LocationValue: 5,
					},
				},
			},
		},

		// Jane Austen - Pride and Prejudice (Public Domain)
		{
			TagNames: []string{"fiction", "classic"},
			Book: entities.Book{
				Title:           "Pride and Prejudice",
				Author:          "Jane Austen",
				Source:          entities.Source{Name: "demo", DisplayName: "Demo Import"},
				PublicationYear: 1813,
				Highlights: []entities.Highlight{
					{
						Text:          "It is a truth universally acknowledged, that a single man in possession of a good fortune, must be in want of a wife.",
						CreatedAt:     now,
						LocationValue: 1,
					},
					{
						Text:          "I declare after all there is no enjoyment like reading! How much sooner one tires of any thing than of a book!",
						CreatedAt:     now,
						LocationValue: 2,
						IsFavorite:    true,
					},
					{
						Text:          "Vanity and pride are different things, though the words are often used synonymously. A person may be proud without being vain.",
						CreatedAt:     now,
						LocationValue: 3,
					},
					{
						Text:          "There is a stubbornness about me that never can bear to be frightened at the will of others. My courage always rises at every attempt to intimidate me.",
						CreatedAt:     now,
						LocationValue: 4,
					},
					{
						Text:          "I cannot fix on the hour, or the spot, or the look, or the words, which laid the foundation. It is too long ago. I was in the middle before I knew that I had begun.",
						CreatedAt:     now,
						LocationValue: 5,
					},
				},
			},
		},

		// Leo Tolstoy - War and Peace (Public Domain)
		{
			TagNames: []string{"fiction", "classic"},
			Book: entities.Book{
				Title:           "War and Peace",
				Author:          "Leo Tolstoy",
				Source:          entities.Source{Name: "demo", DisplayName: "Demo Import"},
				PublicationYear: 1869,
				Highlights: []entities.Highlight{
					{
						Text:          "The two most powerful warriors are patience and time.",
						CreatedAt:     now,
						LocationValue: 1,
						IsFavorite:    true,
					},
					{
						Text:          "Nothing is so necessary for a young man as the company of intelligent women.",
						CreatedAt:     now,
						LocationValue: 2,
					},
					{
						Text:          "We can know only that we know nothing. And that is the highest degree of human wisdom.",
						CreatedAt:     now,
						LocationValue: 3,
					},
					{
						Text:          "If everyone fought for their own convictions there would be no war.",
						CreatedAt:     now,
						LocationValue: 4,
					},
					{
						Text:          "The strongest of all warriors are these two — Time and Patience.",
						CreatedAt:     now,
						LocationValue: 5,
					},
					{
						Text:          "Everything I know, I know only because I love.",
						CreatedAt:     now,
						LocationValue: 6,
					},
				},
			},
		},

		// Fyodor Dostoevsky - Crime and Punishment (Public Domain)
		{
			TagNames: []string{"fiction", "classic"},
			Book: entities.Book{
				Title:           "Crime and Punishment",
				Author:          "Fyodor Dostoevsky",
				Source:          entities.Source{Name: "demo", DisplayName: "Demo Import"},
				PublicationYear: 1866,
				Highlights: []entities.Highlight{
					{
						Text:          "Pain and suffering are always inevitable for a large intelligence and a deep heart.",
						CreatedAt:     now,
						LocationValue: 1,
					},
					{
						Text:          "The soul is healed by being with children.",
						CreatedAt:     now,
						LocationValue: 2,
					},
					{
						Text:          "To go wrong in one's own way is better than to go right in someone else's.",
						CreatedAt:     now,
						LocationValue: 3,
						IsFavorite:    true,
					},
					{
						Text:          "Taking a new step, uttering a new word, is what people fear most.",
						CreatedAt:     now,
						LocationValue: 4,
					},
					{
						Text:          "Man grows used to everything, the scoundrel!",
						CreatedAt:     now,
						LocationValue: 5,
					},
				},
			},
		},

		// Plato - The Republic (Public Domain)
		{
			TagNames: []string{"philosophy", "classic"},
			Book: entities.Book{
				Title:           "The Republic",
				Author:          "Plato",
				Source:          entities.Source{Name: "demo", DisplayName: "Demo Import"},
				PublicationYear: -375,
				Highlights: []entities.Highlight{
					{
						Text:          "The measure of a man is what he does with power.",
						CreatedAt:     now,
						LocationValue: 1,
					},
					{
						Text:          "Opinion is the medium between knowledge and ignorance.",
						CreatedAt:     now,
						LocationValue: 2,
					},
					{
						Text:          "The beginning is the most important part of the work.",
						CreatedAt:     now,
						LocationValue: 3,
						IsFavorite:    true,
					},
					{
						Text:          "Justice in the life and conduct of the State is possible only as first it resides in the hearts and souls of the citizens.",
						CreatedAt:     now,
						LocationValue: 4,
					},
					{
						Text:          "Those who tell the stories rule society.",
						CreatedAt:     now,
						LocationValue: 5,
					},
					{
						Text:          "Good actions give strength to ourselves and inspire good actions in others.",
						CreatedAt:     now,
						LocationValue: 6,
					},
				},
			},
		},

		// Sun Tzu - The Art of War (Public Domain)
		{
			TagNames: []string{"philosophy", "classic"},
			Book: entities.Book{
				Title:           "The Art of War",
				Author:          "Sun Tzu",
				Source:          entities.Source{Name: "demo", DisplayName: "Demo Import"},
				PublicationYear: -500,
				Highlights: []entities.Highlight{
					{
						Text:          "If you know the enemy and know yourself, you need not fear the result of a hundred battles.",
						CreatedAt:     now,
						LocationValue: 1,
						IsFavorite:    true,
					},
					{
						Text:          "In the midst of chaos, there is also opportunity.",
						CreatedAt:     now,
						LocationValue: 2,
					},
					{
						Text:          "The supreme art of war is to subdue the enemy without fighting.",
						CreatedAt:     now,
						LocationValue: 3,
					},
					{
						Text:          "Victorious warriors win first and then go to war, while defeated warriors go to war first and then seek to win.",
						CreatedAt:     now,
						LocationValue: 4,
					},
					{
						Text:          "Appear weak when you are strong, and strong when you are weak.",
						CreatedAt:     now,
						LocationValue: 5,
					},
				},
			},
		},

		// Mary Shelley - Frankenstein (Public Domain)
		{
			TagNames: []string{"fiction", "classic", "science"},
			Book: entities.Book{
				Title:           "Frankenstein",
				Author:          "Mary Shelley",
				Source:          entities.Source{Name: "demo", DisplayName: "Demo Import"},
				PublicationYear: 1818,
				Highlights: []entities.Highlight{
					{
						Text:          "Beware; for I am fearless, and therefore powerful.",
						CreatedAt:     now,
						LocationValue: 1,
					},
					{
						Text:          "Nothing is so painful to the human mind as a great and sudden change.",
						CreatedAt:     now,
						LocationValue: 2,
						IsFavorite:    true,
					},
					{
						Text:          "Life, although it may only be an accumulation of anguish, is dear to me, and I will defend it.",
						CreatedAt:     now,
						LocationValue: 3,
					},
					{
						Text:          "There is something at work in my soul, which I do not understand.",
						CreatedAt:     now,
						LocationValue: 4,
					},
					{
						Text:          "I ought to be thy Adam, but I am rather the fallen angel.",
						CreatedAt:     now,
						LocationValue: 5,
					},
				},
			},
		},

		// Oscar Wilde - The Picture of Dorian Gray (Public Domain)
		{
			TagNames: []string{"fiction", "classic"},
			Book: entities.Book{
				Title:           "The Picture of Dorian Gray",
				Author:          "Oscar Wilde",
				Source:          entities.Source{Name: "demo", DisplayName: "Demo Import"},
				PublicationYear: 1890,
				Highlights: []entities.Highlight{
					{
						Text:          "To define is to limit.",
						CreatedAt:     now,
						LocationValue: 1,
					},
					{
						Text:          "The only way to get rid of a temptation is to yield to it.",
						CreatedAt:     now,
						LocationValue: 2,
						IsFavorite:    true,
					},
					{
						Text:          "I don't want to be at the mercy of my emotions. I want to use them, to enjoy them, and to dominate them.",
						CreatedAt:     now,
						LocationValue: 3,
					},
					{
						Text:          "Experience is merely the name men gave to their mistakes.",
						CreatedAt:     now,
						LocationValue: 4,
					},
					{
						Text:          "Behind every exquisite thing that existed, there was something tragic.",
						CreatedAt:     now,
						LocationValue: 5,
					},
					{
						Text:          "The books that the world calls immoral are books that show the world its own shame.",
						CreatedAt:     now,
						LocationValue: 6,
					},
				},
			},
		},
	}
}

func addVocabularyWords(db *database.Database) {
	words := []struct {
		word       string
		status     entities.WordStatus
		definition string
		pos        string
		example    string
	}{
		{
			word:       "stoicism",
			status:     entities.WordStatusEnriched,
			definition: "The endurance of pain or hardship without the display of feelings and without complaint",
			pos:        "noun",
			example:    "He accepted his fate with remarkable stoicism.",
		},
		{
			word:       "ephemeral",
			status:     entities.WordStatusEnriched,
			definition: "Lasting for a very short time",
			pos:        "adjective",
			example:    "Fame in the modern world is ephemeral.",
		},
		{
			word:       "perspicacious",
			status:     entities.WordStatusEnriched,
			definition: "Having a ready insight into and understanding of things",
			pos:        "adjective",
			example:    "A perspicacious observer of human nature.",
		},
		{
			word:       "sagacity",
			status:     entities.WordStatusEnriched,
			definition: "The quality of being sagacious; wisdom or discernment",
			pos:        "noun",
			example:    "A man of great political sagacity.",
		},
		{
			word:       "equanimity",
			status:     entities.WordStatusEnriched,
			definition: "Mental calmness, composure, and evenness of temper, especially in a difficult situation",
			pos:        "noun",
			example:    "She accepted both success and failure with equanimity.",
		},
		{
			word:   "ameliorate",
			status: entities.WordStatusPending,
		},
		{
			word:   "verisimilitude",
			status: entities.WordStatusPending,
		},
	}

	for _, w := range words {
		word := &entities.Word{
			Word:   w.word,
			Status: w.status,
		}

		if err := db.AddWord(word); err != nil {
			log.Printf("Failed to add word %s: %v", w.word, err)
			continue
		}

		if w.status == entities.WordStatusEnriched && w.definition != "" {
			defs := []entities.WordDefinition{
				{
					WordID:       word.ID,
					PartOfSpeech: w.pos,
					Definition:   w.definition,
					Example:      w.example,
				},
			}
			if err := db.SaveDefinitions(word.ID, defs); err != nil {
				log.Printf("Failed to save definition for %s: %v", w.word, err)
			}
		}

		log.Printf("Added vocabulary word: %s (%s)", w.word, w.status)
	}
}
