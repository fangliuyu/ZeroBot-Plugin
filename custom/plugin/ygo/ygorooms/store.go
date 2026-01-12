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

	engine = control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "游戏王开房工具",
		Help: "-/记录房间 xxx\n  记录个人房间\n" +
			"-/房间\n  输出记录的房间名,如果没有就随机生成一个房间(默认OT房)\n\n" +
			"---想生成指定条件的随机房间可选以下参数----\n\n" +
			"-t\t(开启双打房)\n-m\t(开启BO3房)\n\n" +
			"-tm 0~999\t(每回合时间，单位:分钟)\n-时间 0~999\t(每回合时间，单位:分钟)\n\n" +
			"-lp 0~99999\n-血 0~99999\n\n" +
			"-dr 0~35\t(每回合抽卡数)\n-抽 0~35\t(每回合抽卡数)\n\n" +
			"-st 1~40\t(起手抽卡数)\n-起 1~40\t(起手抽卡数)\n\n" +
			"-mr 1|2|3|新大师|2020 \n-大师 1|2|3|新大师|2020\n\n" +
			"-lf 卡表位号\t(0表示无禁卡)\n-卡表 卡表位号\t(0表示无禁卡)\n\n" +
			"-ot\t(可使用T独, OT混合卡池)\n-nc\t(不检查卡组)\n-ns\t(不洗切卡组)\n\n" +
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
		if err = database.db.Create(serverTable, &dbData{}); err != nil {
			ctx.SendChain(message.Text("[sygoroomsteam] ERROR: ", err))
			return false
		}
		if err = database.db.Create(roomTable, &dbData{}); err != nil {
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

type dbData struct {
	ID   int64
	Info string
}

func (sdb *roomsDB) update(table string, dbInfo dbData) error {
	sdb.Lock()
	defer sdb.Unlock()
	return sdb.db.Insert(table, &dbInfo)
}

func (sdb *roomsDB) find(table string, id int64) (dbInfo dbData, err error) {
	sdb.Lock()
	defer sdb.Unlock()
	if !sdb.db.CanFind(table, "WHERE ID = ?", id) {
		return dbData{}, nil
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

// GameInfo 游戏信息
type GameInfo struct {
	GroupID   int64
	Server    string // 添加服务器信息，避免重复查询
	StartTime time.Time
	RoomInfo
}

type GroupLock struct {
	sync.RWMutex
	rooms map[string]*GameInfo // key: groupID + ":" + roomName
}

// NewGroupLock 创建新的GroupLock
func NewGroupLock() *GroupLock {
	return &GroupLock{
		rooms: make(map[string]*GameInfo),
	}
}

var gameGroup = NewGroupLock()

// AddRoom 添加房间监听
func (g *GroupLock) AddRoom(groupID int64, roomName string, roomInfo RoomInfo, server string) {
	g.Lock()
	defer g.Unlock()

	key := g.generateKey(groupID, roomName)
	g.rooms[key] = &GameInfo{
		RoomInfo:  roomInfo,
		StartTime: time.Now(),
		Server:    server,
		GroupID:   groupID,
	}
}

// RemoveRoom 移除房间监听
func (g *GroupLock) RemoveRoom(groupID int64, roomName string) {
	g.Lock()
	defer g.Unlock()

	key := g.generateKey(groupID, roomName)
	delete(g.rooms, key)
}

// GetRoom 获取房间信息
func (g *GroupLock) GetRoom(groupID int64, roomName string) (*GameInfo, bool) {
	g.RLock()
	defer g.RUnlock()

	key := g.generateKey(groupID, roomName)
	info, ok := g.rooms[key]
	if ok {
		// 返回副本，避免并发修改
		copyInfo := *info
		return &copyInfo, true
	}
	return nil, false
}

// GetAllRooms 获取所有房间的快照
func (g *GroupLock) GetAllRooms() []*GameInfo {
	g.RLock()
	defer g.RUnlock()

	result := make([]*GameInfo, 0, len(g.rooms))
	for _, info := range g.rooms {
		// 返回副本
		copyInfo := *info
		result = append(result, &copyInfo)
	}
	return result
}

// UpdateRoom 更新房间信息
func (g *GroupLock) UpdateRoom(groupID int64, roomName string, updateFunc func(*GameInfo)) bool {
	g.Lock()
	defer g.Unlock()

	key := g.generateKey(groupID, roomName)
	info, ok := g.rooms[key]
	if ok {
		updateFunc(info)
		return true
	}
	return false
}

// generateKey 生成房间key
func (g *GroupLock) generateKey(groupID int64, roomName string) string {
	return string(rune(groupID)) + ":" + roomName
}

// HasRoom 检查房间是否存在
func (g *GroupLock) HasRoom(groupID int64, roomName string) bool {
	g.RLock()
	defer g.RUnlock()

	key := g.generateKey(groupID, roomName)
	_, ok := g.rooms[key]
	return ok
}

// GetRoomsByGroup 获取指定群组的所有房间
func (g *GroupLock) GetRoomsByGroup(groupID int64) []string {
	g.RLock()
	defer g.RUnlock()

	var rooms []string
	for key, info := range g.rooms {
		if info.GroupID == groupID {
			// 从key中提取roomName
			parts := g.parseKey(key)
			if len(parts) == 2 {
				rooms = append(rooms, parts[1])
			}
		}
	}
	return rooms
}

// parseKey 解析key
func (g *GroupLock) parseKey(key string) []string {
	for i, ch := range key {
		if ch == ':' {
			return []string{key[:i], key[i+1:]}
		}
	}
	return []string{}
}
