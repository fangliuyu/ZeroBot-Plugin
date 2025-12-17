// Package ygo 一些关于ygo的插件
package ygo

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/FloatTech/floatbox/web"
)

const (
	defaultApi = "https://.../api/getrooms?"
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

func getApiRooms(server string) (data RoomsApiData, err error) {
	rep, err := web.GetData(server)
	if err != nil {
		return
	}
	err = json.Unmarshal(rep, &data)
	return
}

func (data *RoomsApiData) filterApiRooms(roomName string) RoomInfo {
	for _, room := range data.Rooms {
		if room.RoomName == roomName {
			return room
		}
	}
	return RoomInfo{}
}

func (room *RoomInfo) getGameMode() string {
	switch room.Roommode {
	case 0:
		return "BO1"
	case 1:
		return "BO3"
	case 2:
		return "双打房"
	default:
		return "未知模式"
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
				return "等待中(4=" + strconv.Itoa(num) + ", 别让等待变成为遗憾)"
			}
		case "BO3", "BO1":
			if num < 2 {
				return "等待中(别让等待变成为遗憾)"
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
