// Package custom 注册用户自定义插件于此
package custom

import (
	_ "github.com/FloatTech/ZeroBot-Plugin/custom/plugin/custom"
	_ "github.com/FloatTech/ZeroBot-Plugin/custom/plugin/score"

	_ "github.com/FloatTech/ZeroBot-Plugin/custom/plugin/ygo"

	_ "github.com/FloatTech/ZeroBot-Plugin/custom/plugin/baidufanyi"
	_ "github.com/FloatTech/ZeroBot-Plugin/custom/plugin/cybercat"
)
