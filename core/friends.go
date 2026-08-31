package core

// 本文件：FixIt 友链数据（data/friends.yml）的读取、校验与追加。
// 友链由 layout: friends 的页面渲染，数据项为 YAML map 列表
// （nickname/avatar/url/description，见主题 layouts/friends.html）。

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Friend 一条友链。
type Friend struct {
	Nickname    string `yaml:"nickname" json:"nickname"`
	Avatar      string `yaml:"avatar,omitempty" json:"avatar"`
	URL         string `yaml:"url" json:"url"`
	Description string `yaml:"description,omitempty" json:"description"`
}

func friendsPath(siteDir string) string {
	return filepath.Join(siteDir, "data", "friends.yml")
}

// ListFriends 读取 data/friends.yml；文件不存在视为空列表。
func ListFriends(siteDir string) ([]Friend, error) {
	b, err := os.ReadFile(friendsPath(siteDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var friends []Friend
	if err := yaml.Unmarshal(b, &friends); err != nil {
		return nil, err
	}
	return friends, nil
}

// ValidateFriend 校验友链：昵称与 http(s) 链接必填。
func ValidateFriend(f Friend) error {
	if strings.TrimSpace(f.Nickname) == "" {
		return errors.New("昵称不能为空")
	}
	if !strings.HasPrefix(f.URL, "https://") && !strings.HasPrefix(f.URL, "http://") {
		return errors.New("链接必须是 http(s):// 开头")
	}
	return nil
}

// AddFriend 校验并追加一条友链到 data/friends.yml（yaml 整体重写，
// 数据文件不需要 front matter 那样的外科手术式保留）。
func AddFriend(siteDir string, f Friend) error {
	if err := ValidateFriend(f); err != nil {
		return err
	}
	friends, err := ListFriends(siteDir)
	if err != nil {
		return err
	}
	friends = append(friends, f)
	out, err := yaml.Marshal(friends)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(friendsPath(siteDir)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(friendsPath(siteDir), out, 0o644)
}
