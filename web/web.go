package web

import "embed"

// Assets 内嵌 Dashboard 的模板与静态资源，保证二进制单文件可分发。
//
//go:embed templates static
var Assets embed.FS
