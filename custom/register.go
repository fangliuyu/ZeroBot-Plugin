// Package custom 注册用户自定义插件于此
package custom

import (
	_ "github.com/FloatTech/ZeroBot-Plugin/custom/plugin/yaner" // 自定义人设

	_ "github.com/FloatTech/ZeroBot-Plugin/custom/plugin/score" // 签到

	_ "github.com/FloatTech/ZeroBot-Plugin/custom/plugin/ygo/jihuanshe" // 游戏王插件-集换社
	_ "github.com/FloatTech/ZeroBot-Plugin/custom/plugin/ygo/mycard"    // 游戏王插件-萌卡类
	_ "github.com/FloatTech/ZeroBot-Plugin/custom/plugin/ygo/rooms"     // 游戏王插件-房间读取
	_ "github.com/FloatTech/ZeroBot-Plugin/custom/plugin/ygo/ygocdb"    // 游戏王插件-白鸽查卡

	_ "github.com/FloatTech/ZeroBot-Plugin/custom/plugin/baidufanyi" // 百度翻译

	_ "github.com/FloatTech/ZeroBot-Plugin/custom/plugin/cybercat" // 赛博养猫

	_ "github.com/FloatTech/ZeroBot-Plugin/custom/plugin/wife" // 抽老婆
)
