package analyze_test

import (
	"testing"

	"go-url-shortener/cmd/linter/analyze"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer_Panic(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyze.Analyzer, "panic")
}

func TestAnalyzer_LogFatal(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyze.Analyzer, "logfatal")
}

func TestAnalyzer_OsExit(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyze.Analyzer, "osexit")
}

func TestAnalyzer_MainPkg(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyze.Analyzer, "mainpkg")
}

func TestAnalyzer_Clean(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyze.Analyzer, "clean")
}
