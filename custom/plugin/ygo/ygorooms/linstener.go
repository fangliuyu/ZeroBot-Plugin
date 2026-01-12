// Package ygo 一些关于ygo的插件
package ygo

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FloatTech/floatbox/binary"
	"github.com/FloatTech/floatbox/file"
	zbmath "github.com/FloatTech/floatbox/math"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

var (
	defaultApi = "https://.../api/getrooms"
)

func init() {
	apifile := engine.DataFolder() + "defaultAPI.txt"
	if file.IsExist(apifile) {
		apiInfo, err := os.ReadFile(apifile)
		if err != nil {
			logrus.Errorf("读取defaultAPI.txt失败: %v", err)
			return
		}
		defaultApi = binary.BytesToString(apiInfo)
	}
	engine.OnPrefix("绑定服务器", zero.OnlyGroup, getDB).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		server := strings.TrimSpace(ctx.State["args"].(string))
		if server == "" {
			// 解绑逻辑
			serverData, err := database.find(serverTable, ctx.Event.GroupID)
			if err != nil {
				ctx.SendChain(message.Text("[ygorooms] ERROR: ", err, "\nEXP: 获取绑定服务器失败"))
				return
			}
			if serverData == (dbData{}) {
				ctx.SendChain(message.Text("当前未绑定服务器"))
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
		_, err := GetApiRooms(server)
		if err != nil {
			ctx.SendChain(message.Text("[ygorooms] ERROR: ", err, "\nEXP: 添加失败, 无法访问服务器"))
			return
		}
		// 更新数据库
		info := dbData{
			ID:   ctx.Event.GroupID,
			Info: server,
		}
		if err = database.update(serverTable, info); err != nil {
			ctx.SendChain(message.Text("[ygorooms播报] ERROR: ", err, "\nEXP: 更新数据库失败"))
			return
		}
		ctx.SendChain(message.Text("绑定成功"))
	})
	engine.OnRegex(`^[a-zA-Z,0-9]+\#.*`, zero.OnlyGroup, getDB).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		roomName := strings.TrimSpace(ctx.Event.RawMessage)
		gid := ctx.Event.GroupID
		// 获取服务器信息
		server, err := getServerForGroup(gid)
		if err != nil {
			logrus.Warnln("[ygorooms] 获取服务器失败:", err)
			return
		}
		if server == "" {
			return
		}

		// 检查是否已监听
		if gameGroup.HasRoom(gid, roomName) {
			ctx.SendChain(message.Text("检测到ygo房间: ", roomName, "\n该房间已处于监听状态"))
			return
		}
		newRoomInfo := RoomInfo{
			RoomName: roomName,
		}
		// 添加监听
		gameGroup.AddRoom(gid, roomName, newRoomInfo, server)
		ctx.SendChain(message.Text("检测到ygo房间: ", roomName, "\n开始播报房间状态"))
	})
	engine.OnFullMatch("拉取ygo房间状态", getDB).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		// 异步处理，避免阻塞
		go func() {

			// 获取所有房间的快照
			allRooms := gameGroup.GetAllRooms()
			if len(allRooms) == 0 {
				return
			}
			startTime := time.Now()

			// 按服务器分组，减少API调用
			serverRooms := groupRoomsByServer(allRooms)

			// 为每个服务器创建处理任务
			var wg sync.WaitGroup
			results := make(chan *roomUpdateResult, len(allRooms))

			for server, rooms := range serverRooms {
				wg.Add(1)
				go processServerRooms(server, rooms, results, &wg)
			}

			// 等待所有处理完成
			go func() {
				wg.Wait()
				close(results)
			}()

			// 收集并发送结果
			for result := range results {
				// 更新房间状态
				if result.shouldUpdate {
					gameGroup.UpdateRoom(result.groupID, result.roomName, func(info *GameInfo) {
						info.RoomInfo = *result.newRoomInfo
					})
				}
				// 发送消息
				if result.message != "" {
					ctx.SendGroupMessage(result.groupID, message.Text(result.message))
				}

				// 移除已结束的房间
				if result.shouldRemove {
					gameGroup.RemoveRoom(result.groupID, result.roomName)
				}
			}

			logrus.Infof("房间状态拉取完成，耗时: %v", time.Since(startTime))
		}()
	})
}

// 获取群组的服务器
func getServerForGroup(groupID int64) (string, error) {
	// 先尝试从数据库获取
	infos, err := database.find(serverTable, groupID)
	if err != nil {
		logrus.Warnln("[ygorooms播报] ERROR: ", err)
	}
	if infos.Info != "" {
		return infos.Info, nil
	}
	// 默认服务器
	if groupID == 759851475 || groupID == 1026352282 {
		return defaultApi, nil
	}

	return "", nil
}

// 房间更新结果
type roomUpdateResult struct {
	groupID      int64
	roomName     string
	message      string
	newRoomInfo  *RoomInfo
	shouldUpdate bool
	shouldRemove bool
}

// 按服务器分组房间
func groupRoomsByServer(rooms []*GameInfo) map[string][]*GameInfo {
	serverRooms := make(map[string][]*GameInfo)
	for _, room := range rooms {
		serverRooms[room.Server] = append(serverRooms[room.Server], room)
	}
	return serverRooms
}

// 处理单个服务器的所有房间
func processServerRooms(server string, rooms []*GameInfo, results chan<- *roomUpdateResult, wg *sync.WaitGroup) {
	defer wg.Done()

	// 获取服务器房间数据
	roomsData, err := GetApiRooms(server)
	if err != nil {
		logrus.Errorf("[ygorooms] 获取服务器 %s 数据失败: %v", server, err)
		return
	}

	// 处理每个房间
	for _, roomInfo := range rooms {
		result := processSingleRoom(&roomsData, roomInfo)
		if result != nil {
			results <- result
		}
	}
}

// 处理单个房间
func processSingleRoom(roomsData *RoomsApiData, roomInfo *GameInfo) *roomUpdateResult {
	roomName := roomInfo.RoomInfo.RoomName

	// 查找最新的房间信息
	newRoomInfo := FilterApiRooms(roomsData, roomName)
	if newRoomInfo == nil {
		// 房间不存在，检查是否应该移除
		elapsed := time.Since(roomInfo.StartTime).Minutes()
		if elapsed >= 3 {
			return &roomUpdateResult{
				groupID:      roomInfo.GroupID,
				roomName:     roomName,
				message:      "[ygorooms播报]\n房间 " + roomName + " 已结束决斗\n持续时间 " + strconv.Itoa(int(elapsed)) + " 分钟",
				shouldRemove: true,
			}
		}
		return nil
	}

	// 生成消息
	msg, shouldSend := generateRoomMessage(newRoomInfo, &roomInfo.RoomInfo)
	if !shouldSend {
		newRoomInfo = &roomInfo.RoomInfo // 不更新房间信息
	}

	return &roomUpdateResult{
		groupID:      roomInfo.GroupID,
		roomName:     roomName,
		message:      msg,
		newRoomInfo:  newRoomInfo,
		shouldUpdate: shouldSend,
	}
}

// 生成房间消息
func generateRoomMessage(newRoom, oldRoom *RoomInfo) (string, bool) {
	var msg strings.Builder
	msg.WriteString("[ygorooms播报]\n")
	msg.WriteString("房间: " + newRoom.RoomName + "\n")

	mode := newRoom.getGameMode()
	msg.WriteString("模式: " + mode + "\n")

	status := newRoom.getGameStatus()
	msg.WriteString("当前状态: " + status + "\n")

	msg.WriteString("玩家状态:\n")
	waited := strings.Contains(status, "等待")
	shouldSend := false
	for _, userData := range newRoom.Users {
		userName := userData.Name
		msg.WriteString("[玩家" + strconv.Itoa(userData.Pos) + "] " + userName + " :\n")

		userMsg := ""

		// 查找旧数据中的玩家信息
		var oldUserInfo *UserInfo
		for _, user := range oldRoom.Users {
			if user.Name == userName {
				oldUserInfo = &user
				break
			}
		}

		if oldUserInfo != nil {
			if !waited {
				userMsg, shouldSend = updateMsg(mode, *oldUserInfo, userData)
			}
			if userMsg == "" && userData.Pos < 4 {
				userMsg += "等待中"
			}
		} else {
			if !waited {
				userMsg, shouldSend = updateMsg(mode, UserInfo{}, userData)
			}
			if userMsg == "" {
				userMsg += "加入房间"
			}
		}

		msg.WriteString(userMsg + "\n")
	}

	if status != oldRoom.getGameStatus() {
		shouldSend = true
	}

	return msg.String(), shouldSend
}

func updateMsg(mode string, oldData, newData UserInfo) (newmsg string, shouldSend bool) {
	newmsg = ""
	shouldSend = false

	if newData.Pos < 4 {
		if mode == "BO3" {
			newmsg += " 已赢小局:" + strconv.Itoa(newData.Status.Score) + " "
		}
		if oldData.Status.LP != newData.Status.LP {
			if zbmath.Abs(newData.Status.LP-oldData.Status.LP) > 2000 {
				shouldSend = true
			}
			newmsg += "LP:" + strconv.Itoa(oldData.Status.LP) + "->" + strconv.Itoa(newData.Status.LP)
		} else {
			newmsg += "LP:" + strconv.Itoa(newData.Status.LP)
		}
		if oldData.Status.Cards != newData.Status.Cards {
			if zbmath.Abs(newData.Status.Cards-oldData.Status.Cards) > 5 {
				shouldSend = true
			}
			newmsg += " 场值评估:" + strconv.Itoa(oldData.Status.Cards) + "->" + strconv.Itoa(newData.Status.Cards) + "\n"
		} else {
			newmsg += " 场值评估:" + strconv.Itoa(newData.Status.Cards) + "\n"
		}
	} else {
		newmsg += "观战中"
	}
	return newmsg, shouldSend
}
