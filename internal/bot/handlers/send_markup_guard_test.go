package handlers

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestNoDoubleReplyMarkupInSingleSendCall(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("globbing handler files: %v", err)
	}

	sendCallRx := regexp.MustCompile(`c\.Send\((?:[^()]|\([^()]*\))*\)`)
	navCallRx := regexp.MustCompile(`keyboards\.\w+Navigation\(\)`)
	selectorArgRx := regexp.MustCompile(`(^|[,(]\s*)selector(\s*[,)])`)

	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}

		for _, call := range sendCallRx.FindAllString(string(content), -1) {
			if navCallRx.MatchString(call) && selectorArgRx.MatchString(call) {
				t.Errorf("%s: found a c.Send(...) call combining an inline selector with a keyboards.XNavigation() reply keyboard in one call - only one *telebot.ReplyMarkup is honored per message, silently dropping the other. Use sendPanelWithNav/sendPanelWithNavHTML instead (see navhelper.go). Call: %s", path, call)
			}
		}
	}
}
