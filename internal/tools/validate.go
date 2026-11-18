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

// validatePatchFile 校验 patch_file 的单次精确替换条件。
// 要求 old_text 在文件里恰好出现一次，这样修改结果才是确定的。
func validatePatchFile(raw json.RawMessage, tc Context) error {
	var input struct {
		Path    string `json:"path"`
		OldText string `json:"old_text"`
		NewText string `json:"new_text"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return err
	}
	if strings.TrimSpace(input.Path) == "" {
		return errors.New("path must not be empty")
	}
	if input.OldText == "" {
		return errors.New("old_text must not be empty")
	}
	target := filepath.Join(tc.CWD, input.Path)
	payload, err := os.ReadFile(target)
	if err != nil {
		return errors.New("path does not exist")
	}
	if count := strings.Count(string(payload), input.OldText); count != 1 {
		return errors.New("old_text must occur exactly once, found " + strconv.Itoa(count))
	}
	return nil
}
