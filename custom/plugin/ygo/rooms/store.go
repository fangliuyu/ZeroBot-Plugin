// Package ygo 一些关于ygo的插件
package ygo

import (
	"sync"
	"time"

	fcext "github.com/FloatTech/floatbox/ctxext"
	sql "github.com/FloatTech/sqlite"
	ctrl "github.com/FloatTech/zbpctrl"
	control "github.com/FloatTech/zbputils/control"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

const (
	serverTable = "servers"
	roomTable   = "rooms"
)

var (
	database roomsDB

	engine = control.Register("ygorooms", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "游戏王开房工具",
		Help: "-/记录房间 xxx\n  记录个人房间" +
			"-/房间\n  输出记录的房间名,如果没有就随机生成一个房间(默认OT房)" +
			"---想生成指定条件的随机房间可选以下参数----\n\n" +
			"-t		(开启双打房)\n-m		(开启BO3房)\n\n" +
			"-tm 0~99  (每回合时间，单位:分钟)\n-时间 0~999  (每回合时间，单位:分钟)\n\n" +
			"-lp 0~99999\n-血 0~99999\n\n" +
			"-dr 0~35  (每回合抽卡数)\n-抽 0~35  (每回合抽卡数)\n\n" +
			"-st 1~40  (起手抽卡数)\n-起 1~40  (起手抽卡数)\n\n" +
			"-mr 1|2|3|新大师|2020 \n-大师 1|2|3|新大师|2020\n\n" +
			"-lf 卡表位号  (0表示无禁卡)\n-卡表 卡表位号  (0表示无禁卡)\n\n" +
			"-ot (可使用T独, OT混合卡池)\n-nc (不检查卡组)\n-ns (不洗切卡组)\n\n" +
			"示例:\n/房间 房名 -t 3 -t -lp8000",
		PrivateDataFolder: "ygorooms",
	})

	// 开启并检查数据库链接
	getDB = fcext.DoOnceOnSuccess(func(ctx *zero.Ctx) bool {
		database.db = sql.New(engine.DataFolder() + "rooms.db")
		err := database.db.Open(time.Hour)
		if err != nil {
			ctx.SendChain(message.Text("[ygorooms] ERROR: ", err))
			return false
		}
		if err = database.db.Create(serverTable, &serverDB{}); err != nil {
			ctx.SendChain(message.Text("[sygoroomsteam] ERROR: ", err))
			return false
		}
		if err = database.db.Create(roomTable, &playerDB{}); err != nil {
			ctx.SendChain(message.Text("[ygorooms] ERROR: ", err))
			return false
		}
		return true
	})
)

type roomsDB struct {
	sync.RWMutex
	db sql.Sqlite
}

type serverDB struct {
	ID     int64
	Server string
}

type playerDB struct {
	ID   int64
	Room string
}

func (sdb *roomsDB) update(table string, dbInfo any) error {
	sdb.Lock()
	defer sdb.Unlock()
	return sdb.db.Insert(table, dbInfo)
}

func (sdb *roomsDB) find(table string, id int64) (dbInfo any, err error) {
	sdb.Lock()
	defer sdb.Unlock()
	if !sdb.db.CanFind(table, "WHERE ID = ?", id) {
		return nil, nil
	}
	err = sdb.db.Find(table, &dbInfo, "WHERE ID = ?", id)
	if err == sql.ErrNullResult { // 规避没有该用户数据的报错
		err = nil
	}
	return
}

// del 删除指定数据
func (sdb *roomsDB) del(table string, id int64) error {
	sdb.Lock()
	defer sdb.Unlock()
	return sdb.db.Del(table, "WHERE ID = ?", id)
}
