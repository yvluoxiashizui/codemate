package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// Fingerprint 计算工作区指纹：绝对路径加当前 git 提交。
// 同一份代码在同一位置得到同一个值；分支切换或仓库移动都会改变它。
func Fingerprint(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	abs = filepath.Clean(abs)

	head := gitHead(abs)
	sum := sha256.Sum256([]byte(abs + "\n" + head))
	return hex.EncodeToString(sum[:8])
}

// gitHead 读取 .git/HEAD 指向的提交。不是 git 仓库时返回空串。
func gitHead(root string) string {
	headPath := filepath.Join(root, ".git", "HEAD")
	payload, err := os.ReadFile(headPath)
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(payload))
	ref, found := strings.CutPrefix(content, "ref: ")
	if !found {
		return content
	}
	refPath := filepath.Join(root, ".git", filepath.FromSlash(ref))
	refPayload, err := os.ReadFile(refPath)
	if err != nil {
		return content
	}
	return strings.TrimSpace(string(refPayload))
}
