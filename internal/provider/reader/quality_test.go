package reader

import (
	"strings"
	"testing"
)

func TestContentIssue(t *testing.T) {
	long := strings.Repeat("正文段落，讲述 QUIC 的握手与多路复用。", 100) // 2000+ runes
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "normal long article",
			in:   long,
			want: "",
		},
		{
			name: "zhihu login wall",
			in:   "# 深入剖析 HTTP/3 协议\n\n请您登录后查看更多专业优质内容\n\n打开知乎 App",
			want: "login-walled",
		},
		{
			name: "cloudflare challenge",
			in:   "Just a moment...\nEnable JavaScript and cookies to continue",
			want: "login-walled",
		},
		{
			name: "nav-only shell page",
			in:   "[首页](/) [产品](/p) [关于](/about) [联系](/contact)",
			want: "no content (too short)",
		},
		{
			name: "empty",
			in:   "   ",
			want: "no content (empty)",
		},
		{
			name: "long article mentioning login is not walled",
			in:   long + "\n\n这一节讨论登录后查看订单的交互设计。",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContentIssue(tt.in); got != tt.want {
				t.Errorf("ContentIssue() = %q, want %q", got, tt.want)
			}
		})
	}
}
