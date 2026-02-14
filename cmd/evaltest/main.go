package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/eval/functions"
	"github.com/crystal-mush/gotinymush/pkg/flatfile"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

func main() {
	dbPath := flag.String("db", "", "Path to TinyMUSH flatfile")
	player := flag.Int("player", 1, "DBRef number to use as player context")
	expr := flag.String("e", "", "Expression to evaluate (non-interactive mode)")
	batch := flag.String("batch", "", "File with expressions to evaluate (one per line)")
	flag.Parse()

	var db *gamedb.Database

	if *dbPath != "" {
		fmt.Fprintf(os.Stderr, "Loading database from %s...\n", *dbPath)
		f, err := os.Open(*dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
			os.Exit(1)
		}
		db, err = flatfile.Parse(f)
		f.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing database: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Loaded %d objects, %d attr definitions\n",
			len(db.Objects), len(db.AttrNames))
	} else {
		// Create minimal empty database for testing
		db = gamedb.NewDatabase()
		// Create a minimal God object (#1)
		db.Objects[1] = &gamedb.Object{
			DBRef:    1,
			Name:     "Wizard",
			Location: 0,
			Contents: gamedb.Nothing,
			Exits:    gamedb.Nothing,
			Link:     0,
			Next:     gamedb.Nothing,
			Owner:    1,
			Parent:   gamedb.Nothing,
			Zone:     gamedb.Nothing,
			Flags:    [3]int{int(gamedb.TypePlayer) | gamedb.FlagWizard, 0, 0},
		}
		// Create Room Zero (#0)
		db.Objects[0] = &gamedb.Object{
			DBRef:    0,
			Name:     "Room Zero",
			Location: gamedb.Nothing,
			Contents: 1,
			Exits:    gamedb.Nothing,
			Link:     gamedb.Nothing,
			Next:     gamedb.Nothing,
			Owner:    1,
			Parent:   gamedb.Nothing,
			Zone:     gamedb.Nothing,
			Flags:    [3]int{int(gamedb.TypeRoom), 0, 0},
		}
		fmt.Fprintf(os.Stderr, "Using minimal test database (no flatfile loaded)\n")
	}

	// Set up eval context
	ctx := eval.NewEvalContext(db)
	ctx.Player = gamedb.DBRef(*player)
	ctx.Cause = gamedb.DBRef(*player)
	ctx.Caller = gamedb.DBRef(*player)
	functions.RegisterAll(ctx)

	if *expr != "" {
		// Single expression mode
		result := ctx.Exec(*expr, eval.EvFCheck|eval.EvEval, nil)
		fmt.Println(result)
		return
	}

	if *batch != "" {
		// Batch mode
		f, err := os.Open(*batch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening batch file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			// Format: expression | expected_result (optional)
			parts := strings.SplitN(line, " | ", 2)
			expression := parts[0]
			result := ctx.Exec(expression, eval.EvFCheck|eval.EvEval, nil)

			if len(parts) == 2 {
				expected := parts[1]
				status := "PASS"
				if result != expected {
					status = "FAIL"
				}
				fmt.Printf("[%s] Line %d: %s\n", status, lineNum, expression)
				if status == "FAIL" {
					fmt.Printf("  Expected: %s\n", expected)
					fmt.Printf("  Got:      %s\n", result)
				}
			} else {
				fmt.Printf("Line %d: %s => %s\n", lineNum, expression, result)
			}

			// Reset function counters between expressions
			ctx.FuncInvkCtr = 0
			ctx.FuncNestLev = 0
		}
		return
	}

	// Interactive REPL mode
	fmt.Println("GoTinyMUSH Eval Engine Test Harness")
	fmt.Printf("Player context: #%d\n", *player)
	fmt.Println("Type MUSH expressions to evaluate. Ctrl+C to exit.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("mush> ")
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if line == "" {
			continue
		}
		if line == "quit" || line == "exit" {
			break
		}

		// Reset counters each eval
		ctx.FuncInvkCtr = 0
		ctx.FuncNestLev = 0

		result := ctx.Exec(line, eval.EvFCheck|eval.EvEval, nil)
		fmt.Println(result)

		// Show any notifications
		if len(ctx.Notifications) > 0 {
			for _, n := range ctx.Notifications {
				fmt.Printf("  [notify #%d]: %s\n", n.Target, n.Message)
			}
			ctx.Notifications = nil
		}
	}
}
