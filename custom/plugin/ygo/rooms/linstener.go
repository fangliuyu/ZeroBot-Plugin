// Package ygo 一些关于ygo的插件
package ygo

import (
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FloatTech/floatbox/binary"
	"github.com/FloatTech/floatbox/file"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

// GameInfo 游戏信息
type GameInfo struct {
	RoomInfo   RoomInfo
	StartTime  time.Time
	UpdateTime int
}

var (
	defaultApi = "https://.../api/getrooms"
	gameGroup  sync.Map
	gameRoom   sync.Map
)

func init() {
	apifile := engine.DataFolder() + "defaultAPI.txt"
	if file.IsExist(apifile) {
		apiInfo, err := os.ReadFile(apifile)
		if err != nil {
			panic(err)
		}
		defaultApi = binary.BytesToString(apiInfo)
	}
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
			ctx.SendChain(message.Text("[ygorooms播报] ERROR: ", err, "\nEXP: 更新数据库失败"))
			return
		}
		ctx.SendChain(message.Text("绑定成功"))
	})
	engine.OnRegex(`^[a-zA-Z,0-9]+\#.*`, zero.OnlyGroup, getDB).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		roomName := strings.TrimSpace(ctx.Event.RawMessage)
		gid := ctx.Event.GroupID
		// 检测是否有服务器绑定
		infos, err := database.find(serverTable, gid)
		if err != nil {
			logrus.Warnln("[ygorooms播报] ERROR: ", err)
			return
		}
		if infos == nil {
			infos = serverDB{}
		}
		infosData := infos.(serverDB)
		if infosData.Server == "" && (gid != 759851475 && gid != 1026352282) {
			return
		}
		// 添加监听
		value, _ := gameGroup.LoadOrStore(gid, []string{})
		roomlist := value.([]string)
		if slices.Contains(roomlist, roomName) {
			ctx.SendChain(message.Text("检测到ygo房间: ", roomName, "\n该房间已处于监听状态"))
			return
		}
		roomlist = append(roomlist, roomName)
		gameGroup.Store(gid, roomlist)
		ctx.SendChain(message.Text("检测到ygo房间: ", roomName, "\n开始播报房间状态"))

		if infosData.Server == "" && (gid == 759851475 || gid == 1026352282) {
			infosData.Server = defaultApi
		}
		rooms, err := getApiRooms(infosData.Server)
		if err != nil {
			ctx.SendChain(message.Text("[steam] ERROR: ", err))
			return
		}
		newData := rooms.filterApiRooms(roomName)
		newRoom := GameInfo{
			RoomInfo:  newData,
			StartTime: time.Now(),
		}
		gameRoom.Store(roomName, newRoom)
	})
	engine.OnFullMatch("拉取ygo房间状态", getDB).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		gameGroup.Range(func(key, value any) bool {
			gid := key.(int64)
			roomlist := value.([]string)
			infos, err := database.find(serverTable, gid)
			if err != nil {
				logrus.Warnln("[ygorooms播报] ERROR: ", err)
				return true
			}
			if infos == nil {
				infos = serverDB{}
			}
			infosData := infos.(serverDB)
			if infosData.Server == "" && (gid != 759851475 && gid != 1026352282) {
				return true
			} else if infosData.Server == "" && (gid == 759851475 || gid == 1026352282) {
				infosData.Server = defaultApi
			}
			rooms, err := getApiRooms(infosData.Server)
			if err != nil {
				logrus.Warnln("[ygorooms播报] ERROR: ", err)
				return true
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
					if elapsed < 3 {
						gameRoom.Store(roomName, roomInfo)
						continue
					}
					// 房间结束，发送消息并删除监听
					gameRoom.Delete(roomName)
					delList = append(delList, roomName)
					ctx.SendGroupMessage(gid, message.Text("[ygorooms播报]\n房间 ", roomName, " 已结束决斗\n持续时间 ", int(elapsed), " 分钟"))
					continue
				}
				roomData, ok := gameRoom.Load(roomName)
				if !ok {
					return true
				}
				data := roomData.(GameInfo)
				var msg strings.Builder
				olderData := data.RoomInfo
				if olderData.Istart == newData.Istart {
					// 相同回合，判断更新时间
					data.UpdateTime++
					gameRoom.Store(roomName, data)
					if data.UpdateTime%2 != 0 {
						continue
					}
				} else {
					data.UpdateTime = 0
				}
				// 状态变化，发送消息
				msg.WriteString("[ygorooms播报]\n房间: " + newData.RoomName + "\n")
				mode := newData.getGameMode()
				msg.WriteString("模式: " + mode + "\n")
				status := newData.getGameStatus()
				msg.WriteString("当前状态: " + status + "\n")
				msg.WriteString("玩家状态:\n")
				waited := strings.Contains(status, "等待")
				for _, userData := range newData.Users {
					userName := userData.Name
					msg.WriteString("[玩家" + strconv.Itoa(userData.Pos) + "] " + userName + " :\n")
					newmsg := ""
					for _, user := range olderData.Users {
						if userName == user.Name {
							if !waited {
								newmsg = updateMsg(mode, user, userData)
							}
							break
						}
					}
					if newmsg == "" {
						if !waited {
							if userData.Pos < 3 {
								if mode == "BO3" {
									newmsg += " 已赢小局:" + strconv.Itoa(userData.Status.Score) + " "
								}
								newmsg += "LP:" + strconv.Itoa(userData.Status.LP) + " 场值评估:" + strconv.Itoa(userData.Status.Cards) + "\n"
							} else {
								newmsg += "观战中"
							}
						} else {
							newmsg += " 加入房间\n"
						}
					}
					msg.WriteString(newmsg)
				}
				// 更新数据
				data.RoomInfo = newData
				gameRoom.Store(roomName, data)
				ctx.SendGroupMessage(gid, message.Text(msg.String()))
			}
			newRoomList := []string{}
			for _, name := range roomlist {
				// 从监听列表删除
				if slices.Contains(delList, name) {
					continue
				}
				newRoomList = append(newRoomList, name)
			}
			gameGroup.Store(gid, newRoomList)
			return true
		})
	})
}

func updateMsg(mode string, oldData, newData UserInfo) string {
	newmsg := ""
	if newData.Pos < 3 {
		if mode == "BO3" {
			newmsg += " 已赢小局:" + strconv.Itoa(newData.Status.Score) + " "
		}
		if oldData.Status.LP != newData.Status.LP {
			newmsg += "LP:" + strconv.Itoa(oldData.Status.LP) + "->" + strconv.Itoa(newData.Status.LP)
		} else {
			newmsg += "LP:" + strconv.Itoa(newData.Status.LP)
		}
		if oldData.Status.Cards != newData.Status.Cards {
			newmsg += " 场值评估:" + strconv.Itoa(oldData.Status.Cards) + "->" + strconv.Itoa(newData.Status.Cards) + "\n"
		} else {
			newmsg += " 场值评估:" + strconv.Itoa(newData.Status.Cards) + "\n"
		}
	} else {
		newmsg += "观战中"
	}
	return newmsg
}
