package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/khordestan/anime-intelligence/internal/modules/intelligence"
)

func run(input io.Reader, output io.Writer, k int) error {
	const maxInput = 8 << 20
	payload, err := io.ReadAll(io.LimitReader(input, maxInput+1))
	if err != nil {
		return err
	}
	if len(payload) > maxInput {
		return errors.New("evaluation input exceeds 8 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var suite intelligence.EvaluationSuite
	if decoder.Decode(&suite) != nil || decoder.Decode(new(any)) != io.EOF {
		return errors.New("expected one evaluation suite JSON object")
	}
	report, err := intelligence.EvaluatePersonalMatch(suite, k)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func main() {
	k := flag.Int("k", 10, "number of ranked positions to evaluate")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "pass the evaluation JSON on standard input")
		os.Exit(2)
	}
	if err := run(os.Stdin, os.Stdout, *k); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
