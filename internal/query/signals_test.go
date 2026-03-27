package query

import (
	"strings"
	"testing"
)

func TestExtractSignals(t *testing.T) {
	t.Run("extract go", func(t *testing.T) {
		src := `
		package main

        	func main() {
        		LoadConfig()
        	}
        	`

		out := ExtractSignals("main.go", []byte(src))
		if !strings.Contains(out, "LoadConfig") {
			t.Errorf("expected Go extraction, got:\n%s", out)
		}
	})

	t.Run("extract text", func(t *testing.T) {
		src := `
		function loadConfig() {
			parseJSON()
		}
        	`

		out := ExtractSignals("file.js", []byte(src))
		if !strings.Contains(out, "parseJSON") {
			t.Errorf("expected text extraction, got:\n%s", out)
		}
	})
}

func Test_extractGoSignals(t *testing.T) {
	t.Run("extract indentifiers", func(t *testing.T) {
		src := `
		package main

		func main() {
			cfg := LoadConfig()
			db := ConnectDB(cfg)
			RunServer(db)
		}
		`

		out := extractGoSignals([]byte(src))
		expected := []string{"LoadConfig", "ConnectDB", "RunServer"}
		for _, e := range expected {
			if !strings.Contains(out, e) {
				t.Errorf("expected output to contain %q, got:\n%s", e, out)
			}
		}
	})

	t.Run("extract method calls", func(t *testing.T) {
		src := `
		package main

		func main() {
			client.DoRequest()
			logger.Info("hello")
		}
		`

		out := extractGoSignals([]byte(src))
		expected := []string{"DoRequest", "Info"}
		for _, e := range expected {
			if !strings.Contains(out, e) {
				t.Errorf("expected method call %q to be extracted, got:\n%s", e, out)
			}
		}
	})

	t.Run("extract imports", func(t *testing.T) {
		src := `
		package main

		import (
			"client"
			"logger"
		)

		func main() {
			client.DoRequest()
			logger.Info("hello")
		}
		`

		out := extractGoSignals([]byte(src))
		expected := []string{"client", "logger", "DoRequest", "Info"}
		for _, e := range expected {
			if !strings.Contains(out, e) {
				t.Errorf("expected method call %q to be extracted, got:\n%s", e, out)
			}
		}
	})

	t.Run("dedups", func(t *testing.T) {
		src := `
		package main

		func main() {
			client.DoRequest()
			client.DoRequest()
			client.DoRequest()
		}
		`

		out := extractGoSignals([]byte(src))
		count := strings.Count(out, "DoRequest")
		if count != 1 {
			t.Errorf("expected 'Dorequest' once, got %d occurrences:\n%s", count, out)
		}
	})
}
func Test_extractTextSignals(t *testing.T) {
	t.Run("extract indentifiers, non-go", func(t *testing.T) {
		src := `
		function loadConfig(path) {
		  return parseJSON(readFile(path))
		}
		`

		out := extractTextSignals([]byte(src))
		expected := []string{"loadConfig", "parseJSON", "readFile"}
		for _, e := range expected {
			if !strings.Contains(out, e) {
				t.Errorf("expected output to contain %q, got:\n%s", e, out)
			}
		}
	})

	t.Run("dedups", func(t *testing.T) {
		src := `
		loadConfig()
		loadConfig()
		loadConfig()
		`

		out := extractTextSignals([]byte(src))
		count := strings.Count(out, "loadConfig")
		if count != 1 {
			t.Errorf("expected 'loadConfig' once, got %d occurrences:\n%s", count, out)
		}
	})
}
