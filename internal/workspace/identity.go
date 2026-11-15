package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
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

// Freshness 返回文件的修改时间与大小指纹，用于判断检查点里的文件是否被改过。
// 文件不存在时返回 "missing"。
func Freshness(root, path string) string {
	target := path
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, path)
	}
	info, err := os.Stat(target)
	if err != nil {
		return "missing"
	}
	return strconv.FormatInt(info.ModTime().UnixNano(), 10) + ":" + strconv.FormatInt(info.Size(), 10)
}
