// Package ygo 一些关于ygo的插件
package ygo

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/FloatTech/floatbox/web"
)

type RoomsApiData struct {
	Rooms []RoomInfo `json:"rooms"`
}

type RoomInfo struct {
	RoomID   string     `json:"roomid"`
	RoomName string     `json:"roomname"`
	Roommode int        `json:"roommode"`
	Needpass string     `json:"needpass"`
	Users    []UserInfo `json:"users"`
	Istart   string     `json:"istart"`
}

type UserInfo struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	IP     string     `json:"ip"`
	Status UserStatus `json:"status"`
	Pos    int        `json:"pos"`
}

type UserStatus struct {
	Score int `json:"score"`
	LP    int `json:"lp"`
	Cards int `json:"cards"`
}

// GetApiRooms 获取API房间数据
func GetApiRooms(server string) (data RoomsApiData, err error) {
	rep, err := web.GetData(server)
	if err != nil {
		return
	}
	err = json.Unmarshal(rep, &data)
	return
}

// FilterApiRooms 过滤API房间
func FilterApiRooms(data *RoomsApiData, roomName string) *RoomInfo {
	for _, room := range data.Rooms {
		if room.RoomName == roomName {
			return &room
		}
	}
	return nil
}

func (room *RoomInfo) getGameMode() string {
	switch room.Roommode {
	case 0:
		return "BO1"
	case 1:
		return "BO3"
	case 2:
		return "2V2"
	default:
		return "Unknown"
	}
}

func (room *RoomInfo) getUserNumber() int {
	return len(room.Users)
}

func (room *RoomInfo) getGameStatus() string {
	mode := room.getGameMode()
	num := room.getUserNumber()
	status := room.Istart
	if status == "wait" {
		switch mode {
		case "双打房":
			if num < 4 {
				return "等待中(4=" + strconv.Itoa(num) + ")"
			} else {
				return "等待中"
			}
		default:
			return "等待中"
		}
	}
	data, err := parseKeyValue(status)
	if err != nil {
		return status
	}
	if mode == "BO3" {
		return "第" + data["Duel"] + "局的第" + data["Turn"] + "回合"
	}
	return "第" + data["Turn"] + "回合"
}

func parseKeyValue(s string) (map[string]string, error) {
	result := make(map[string]string)
	parts := strings.Split(s, " ")

	for _, part := range parts {
		kv := strings.Split(part, ":")
		if len(kv) != 2 {
			continue // 跳过无效格式
		}
		name := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		result[name] = value
	}
	return result, nil
}
