package version

import "fmt"

// 這些值會在 release build 時由 linker flags 注入。
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// String 回傳適合顯示給使用者的版本資訊。
func String(name string) string {
	return fmt.Sprintf("%s %s (commit=%s, date=%s)", name, Version, Commit, Date)
}
