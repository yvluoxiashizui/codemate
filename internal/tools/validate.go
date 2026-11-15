package tools

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// 校验用的上限，与执行阶段保持一致。
const (
	maxReadLimit   = 20000
	maxReadOffset  = 1 << 30
	runTimeoutHint = 120
)

// validateListFiles 校验 list_files 的目标是目录。
func validateListFiles(raw json.RawMessage, tc Context) error {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return err
	}
	target := input.Path
	if target == "" {
		target = "."
	}
	info, err := os.Stat(filepath.Join(tc.CWD, target))
	if err != nil {
		return errors.New("path does not exist")
	}
	if !info.IsDir() {
		return errors.New("path is not a directory")
	}
	return nil
}

// validateReadFile 校验 read_file 的目标是文件且行范围合法。
func validateReadFile(raw json.RawMessage, tc Context) error {
	var input struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return err
	}
	if strings.TrimSpace(input.Path) == "" {
		return errors.New("path must not be empty")
	}
	info, err := os.Stat(filepath.Join(tc.CWD, input.Path))
	if err != nil {
		return errors.New("path does not exist")
	}
	if info.IsDir() {
		return errors.New("path is not a file")
	}
	if input.Offset < 0 {
		return errors.New("offset must not be negative")
	}
	if input.Limit < 0 || input.Limit > maxReadLimit {
		return errors.New("limit must be in [0, " + strconv.Itoa(maxReadLimit) + "]")
	}
	return nil
}

// validateGrepFiles 校验检索条件不为空。
func validateGrepFiles(raw json.RawMessage, tc Context) error {
	var input struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return err
	}
	if strings.TrimSpace(input.Pattern) == "" {
		return errors.New("pattern must not be empty")
	}
	if input.Path != "" {
		if _, err := os.Stat(filepath.Join(tc.CWD, input.Path)); err != nil {
			return errors.New("path does not exist")
		}
	}
	return nil
}

// validateRunCommand 校验命令非空、超时在允许区间。
func validateRunCommand(raw json.RawMessage, tc Context) error {
	var input struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return err
	}
	if strings.TrimSpace(input.Command) == "" {
		return errors.New("command must not be empty")
	}
	if input.Timeout < 0 || input.Timeout > runTimeoutHint {
		return errors.New("timeout must be in [0, " + strconv.Itoa(runTimeoutHint) + "]")
	}
	return nil
}

// validateWriteFile 校验不会把目录当文件写。
func validateWriteFile(raw json.RawMessage, tc Context) error {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return err
	}
	if strings.TrimSpace(input.Path) == "" {
		return errors.New("path must not be empty")
	}
	if info, err := os.Stat(filepath.Join(tc.CWD, input.Path)); err == nil && info.IsDir() {
		return errors.New("path is a directory")
	}
	return nil
}

// validatePatchFile 校验每条替换的检索片段都不为空。
func validatePatchFile(raw json.RawMessage, tc Context) error {
	var input struct {
		Path         string `json:"path"`
		Replacements []struct {
			Search string `json:"search"`
		} `json:"replacements"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return err
	}
	if strings.TrimSpace(input.Path) == "" {
		return errors.New("path must not be empty")
	}
	if len(input.Replacements) == 0 {
		return errors.New("replacements must not be empty")
	}
	for index, item := range input.Replacements {
		if item.Search == "" {
			return errors.New("replacement " + strconv.Itoa(index+1) + ": search must not be empty")
		}
	}
	return nil
}
