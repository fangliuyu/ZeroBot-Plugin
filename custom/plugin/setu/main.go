// Package setutime 来份涩图
package setutime

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	fcext "github.com/FloatTech/floatbox/ctxext"
	sql "github.com/FloatTech/sqlite"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

var (
	NowList []string = DefaultList
	engine           = control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "涩图",
		Help: "- 来份涩图 [caption]\n" +
			"- 添加涩图 [caption] [P站图片ID]\n" +
			"- 删除涩图 [caption] [P站图片ID]\n" +
			"- 涩图列表",
		PrivateDataFolder: "setu",
	})

	InitImgPool = fcext.DoOnceOnSuccess(func(ctx *zero.Ctx) bool {
		ImgPool.db = sql.New(engine.DataFolder() + "pixiv.db")
		err := ImgPool.db.Open(time.Hour)
		if err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
			return false
		}
		NowList = ImgPool.List()
		for _, imgtype := range NowList {
			if err := ImgPool.db.Create(imgtype, &Illust{}); err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return false
			}
		}
		return true
	})
)

func init() { // 插件主体
	engine.OnRegex(`^来份涩图\s*(.*)$`, InitImgPool).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(func(ctx *zero.Ctx) {

			var imgtype = ctx.State["regex_matched"].([]string)[1]

			if imgtype == "" {
				imgtype = "游戏王"
			}
			ok := false
			for _, v := range ImgPool.List() {
				if imgtype == v {
					ok = true
				}
			}
			if !ok {
				ctx.SendChain(message.Text("ERROR: 分类不存在"))
				return
			}

			// 补充池子
			go AddAPItoPool(imgtype)

			// 如果没有缓存，轮询等待
			if ImgPool.Size(imgtype) == 0 {
				ctx.SendChain(message.Text("INFO: 正在填充弹药......"))

				timeout := time.After(30 * time.Second)
				ticker := time.NewTicker(500 * time.Millisecond)
				defer ticker.Stop()

				for ImgPool.Size(imgtype) == 0 {
					select {
					case <-timeout:
						ctx.SendChain(message.Text("ERROR: 等待填充，请稍后再试......"))
						return
					case <-ticker.C:
						// 继续循环检查
					}
				}
			}

			// 从缓冲池里抽一张
			imgPath := ImgPool.Pop(imgtype)
			if imgPath == "" {
				ctx.SendChain(message.Text("ERROR: 获取图片失败"))
				return
			}
			pic, err := os.ReadFile(imgPath)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]", err))
				return
			}
			m := (message.ImageBytes(pic))
			if id := ctx.Send(m).ID(); id == 0 {
				ctx.SendChain(message.Text("ERROR: 可能被风控了"))
			}
		})

	// engine.OnRegex(`^添加涩图\s*([^0-9\s]+)\s*(\d+)$`, InitImgPool).SetBlock(true).
	// 	Handle(func(ctx *zero.Ctx) {
	// 		var (
	// 			imgtype  = ctx.State["regex_matched"].([]string)[1]
	// 			id, _   = strconv.ParseInt(ctx.State["regex_matched"].([]string)[2], 10, 64)
	// 		)
	// 		data, err := GetAPIData(fmt.Sprintf(sreachURL, "tag="+keyworld))
	// 		if err != nil {
	// 			ctx.SendChain(message.Text("ERROR: ", err))
	// 			return
	// 		}
	// 		if len(data) == 0 {
	// 			ctx.SendChain(message.Text("ERROR: 没有找到相关图片"))
	// 			return
	// 		}
	// 		illusts, err := TransListToIllust(data)
	// 		if err != nil {
	// 			ctx.SendChain(message.Text("ERROR: ", err))
	// 			return
	// 		}
	// 		for _, illust := range illusts {
	// 			err := ImgPool.AddLocal(imgtype, illust.Pid, illust.ImageUrls...)
	// 			if err != nil {
	// 				ctx.SendChain(message.Text("ERROR: ", err))
	// 				return
	// 			}
	// 		}
	// 		ctx.SendChain(message.Text("成功向分类", imgtype, "添加图片", id))
	// 	})

	engine.OnRegex(`^删除涩图\s*([^0-9\s]+)\s*(\d+)$`, InitImgPool,
		fcext.ValueInList(func(ctx *zero.Ctx) string { return ctx.State["regex_matched"].([]string)[1] }, ImgPool),
		zero.SuperUserPermission).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		var (
			imgtype = ctx.State["regex_matched"].([]string)[1]
			id, _   = strconv.ParseInt(ctx.State["regex_matched"].([]string)[2], 10, 64)
		)
		// 查询数据库
		if err := ImgPool.Remove(imgtype, id); err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
			return
		}
		ctx.SendChain(message.Text("删除成功"))
	})

	// 查询数据库涩图数量
	engine.OnFullMatch("涩图列表", InitImgPool).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			state := []string{"[SetuTime]"}
			ImgPool.dbmu.RLock()
			defer ImgPool.dbmu.RUnlock()
			for _, imgtype := range ImgPool.List() {
				num, err := ImgPool.db.Count(imgtype)
				if err != nil {
					num = 0
				}
				state = append(state, "\n")
				state = append(state, imgtype)
				state = append(state, ": ")
				state = append(state, fmt.Sprintf("%d", num))
			}
			ctx.SendChain(message.Text(strings.Join(state, "")))
		})
}
