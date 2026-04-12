package query

import "testing"

func TestClassifyQuery(t *testing.T) {
	testCases := map[string]struct {
		query    string
		expected bool
	}{
		// --- Strong identifier anchors ---
		"camel caseidentifier": {
			query:    "What does SearchCommand do?",
			expected: true,
		},
		"snake case identifier": {
			query:    "explain load_config function",
			expected: true,
		},
		"mixed identifier short": {
			query:    "QueryCommand",
			expected: true,
		},

		// --- File references ---
		"go file reference": {
			query:    "summarize command/query.go",
			expected: true,
		},
		"json file reference": {
			query:    "what is in config.json",
			expected: true,
		},

		// --- Package / path references ---
		"package path": {
			query:    "how does internal/store work",
			expected: true,
		},

		// --- Error messages ---
		"panic error": {
			query:    "panic: nil pointer dereference",
			expected: true,
		},
		"undefined error": {
			query:    "undefined: QueryCommand",
			expected: true,
		},

		// --- Code vocabulary (soft anchors) ---
		"mentions command": {
			query:    "what does the search command do",
			expected: true,
		},
		"mentions function": {
			query:    "which function loads the config",
			expected: true,
		},
		"mentions struct": {
			query:    "explain the store struct",
			expected: true,
		},

		// --- Vague / exploratory ---
		"generic question": {
			query:    "what is this",
			expected: false,
		},
		"how does this_work": {
			query:    "how does this work",
			expected: false,
		},
		"generic cli usage": {
			query:    "how do I use this cli",
			expected: false,
		},
		"completely unrelated": {
			query:    "who was the king of wales",
			expected: false,
		},

		// --- Edge cases ---
		"empty query": {
			query:    "",
			expected: false,
		},
		"whitespace only": {
			query:    "   ",
			expected: false,
		},
		"short noise": {
			query:    "help",
			expected: false,
		},

		// --- Tricky borderline cases ---
		"long but no anchor": {
			query:    "how can I improve performance of this application",
			expected: false,
		},
		"quoted string": {
			query:    `what does "SearchCommand" do`,
			expected: true,
		},
		"path like but not code": {
			query:    "go to /home directory",
			expected: false,
		},
		"natural language command": {
			query:    "explain the query command logic",
			expected: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := IsSearchable(tc.query)
			if got != tc.expected {
				t.Fatalf("expected %v, got %v for query: %q", tc.expected, got, tc.query)
			}
		})
	}
}
