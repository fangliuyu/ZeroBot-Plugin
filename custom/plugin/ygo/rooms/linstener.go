// Package ygo 一些关于ygo的插件
package ygo

import (
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

// GameInfo 游戏信息
type GameInfo struct {
	RoomInfo  RoomInfo
	StartTime time.Time
}

var (
	gameGroup sync.Map
	gameRoom  sync.Map
)

func init() {
	engine.OnPrefix("绑定服务器", zero.OnlyGroup, getDB).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		server := strings.TrimSpace(ctx.State["args"].(string))
		if server == "" {
			data, err := database.find(serverTable, ctx.Event.GroupID)
			if err != nil {
				ctx.SendChain(message.Text("[ygorooms] ERROR: ", err, "\nEXP: 获取绑定服务器失败"))
				return
			}
			serverData := data.(serverDB)
			if serverData.Server == "" {
				ctx.SendChain(message.Text("当前未绑定服务器，请使用命令绑定服务器"))
				return
			}
			err = database.del(serverTable, ctx.Event.GroupID)
			if err != nil {
				ctx.SendChain(message.Text("[ygorooms] ERROR: ", err, "\nEXP: 解绑服务器失败"))
				return
			}
			ctx.SendChain(message.Text("解绑成功"))
			return
		}
		// 验证服务器是否可以访问
		_, err := getApiRooms(server)
		if err != nil {
			ctx.SendChain(message.Text("[ygorooms] ERROR: ", err, "\nEXP: 添加失败, 无法访问服务器"))
			return
		}
		// 更新数据库
		info := serverDB{
			ID:     ctx.Event.GroupID,
			Server: server,
		}
		if err = database.update(serverTable, &info); err != nil {
			ctx.SendChain(message.Text("[steam] ERROR: ", err, "\nEXP: 更新数据库失败"))
			return
		}
		ctx.SendChain(message.Text("绑定成功"))
	})
	engine.OnRegex(`^[a-zA-Z,0-9]+\#.*`, zero.OnlyGroup, getDB).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		roomName := strings.TrimSpace(ctx.Event.RawMessage)
		gid := ctx.Event.GroupID
		value, _ := gameGroup.LoadOrStore(gid, []string{})
		roomlist := value.([]string)
		if slices.Contains(roomlist, roomName) {
			ctx.SendChain(message.Text("检测到ygo房间: ", roomName, "\n该房间已处于监听状态"))
			return
		}
		roomlist = append(roomlist, roomName)
		gameGroup.Store(gid, roomlist)
		ctx.SendChain(message.Text("检测到ygo房间: ", roomName, "\n开始播报房间状态"))
	})
	engine.OnFullMatch("拉取ygo房间状态", getDB).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		gameGroup.Range(func(key, value any) bool {
			gid := key.(int64)
			roomlist := value.([]string)
			infos, err := database.find(serverTable, gid)
			if err != nil {
				logrus.Warnln("[steam] ERROR: ", err)
				return false
			}
			if infos == nil {
				infos = serverDB{}
			}
			infosData := infos.(serverDB)
			if infosData.Server == "" && (gid != 759851475 && gid != 1026352282) {
				return false
			} else if infosData.Server == "" && (gid == 759851475 || gid == 1026352282) {
				infosData.Server = defaultApi
			}
			rooms, err := getApiRooms(infosData.Server)
			if err != nil {
				logrus.Warnln("[steam] ERROR: ", err)
				return false
			}
			delList := []string{}
			for _, roomName := range roomlist {
				newData := rooms.filterApiRooms(roomName)
				if newData.RoomID == "" {
					roomData, ok := gameRoom.Load(roomName)
					if !ok {
						roomData = GameInfo{
							RoomInfo:  newData,
							StartTime: time.Now(),
						}
					}
					roomInfo := roomData.(GameInfo)
					now := time.Now()
					elapsed := now.Sub(time.Unix(roomInfo.StartTime.Unix(), 0)).Minutes()
					gameRoom.Delete(roomName)
					delList = append(delList, roomName)
					ctx.SendGroupMessage(gid, message.Text("[ygorooms播报] 房间 ", roomName, " 已结束决斗, 持续时间 ", int(elapsed), " 分钟"))
					continue
				}
				roomData, ok := gameRoom.Load(roomName)
				if !ok {
					return true
				}
				data := roomData.(GameInfo)
				msg := ""
				olderData := data.RoomInfo
				// if olderData.Istart == newData.Istart {
				// 	continue
				// }
				// 状态变化，发送消息
				msg += "[ygorooms播报] 房间 " + newData.RoomName + " :\n"
				mode := newData.getGameMode()
				msg += "模式: " + mode + "\n"
				status := newData.getGameStatus()
				msg += "当前状态: " + status + "\n"
				msg += "玩家状态:\n"
				waited := strings.Contains(status, "等待")
				for i, userData := range newData.Users {
					userName := userData.Name
					msg += "[玩家" + strconv.Itoa(i+1) + "] " + userName + " :"
					newmsg := ""
					for _, user := range olderData.Users {
						if userName == user.Name {
							if !waited {
								if mode == "BO3" {
									newmsg += " 已赢小局:" + strconv.Itoa(userData.Status.Score) + " "
								}
								newmsg += "当前LP:" + strconv.Itoa(userData.Status.LP) + "\n"
							} else {
								if userData.Pos == 1 {
									newmsg += "已准备\n"
								} else {
									newmsg += "未准备\n"
								}
							}
							break
						}
					}
					if newmsg == "" {
						newmsg = " 加入房间,"
						if userData.Pos == 1 {
							newmsg += "已准备\n"
						} else {
							newmsg += "未准备\n"
						}
					}
					msg += newmsg
				}
				// 更新数据
				data.RoomInfo = newData
				gameRoom.Store(roomName, data)
				ctx.SendGroupMessage(gid, message.Text(msg))
			}
			newRoomList := []string{}
			for _, name := range delList {
				// 从监听列表删除
				if !slices.Contains(roomlist, name) {
					newRoomList = append(newRoomList, name)
				}
			}
			gameGroup.Store(gid, newRoomList)
			return true
		})
	})
}
