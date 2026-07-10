// Package setutime 来份涩图
package setutime

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/FloatTech/floatbox/web"
)

const (
	loliconAPI = "https://api.lolicon.app/setu/v2"
	SreachURL  = loliconAPI + "?num=20&r18=2&excludeAI=1%s"
)

type LoliconData struct {
	Error string    `json:"error"`
	Data  []PixInfo `json:"data"`
}

type PixInfo struct {
	Pid        int64    `json:"pid"`
	P          int      `json:"p"`
	UID        int64    `json:"uid"`
	Title      string   `json:"title"`
	Author     string   `json:"author"`
	R18        bool     `json:"r18"`
	Width      int      `json:"width"`
	Height     int      `json:"height"`
	Tags       []string `json:"tags"`
	Ext        string   `json:"ext"`
	AiType     int      `json:"aiType"`
	UploadDate int64    `json:"uploadDate"`
	Urls       struct {
		Original string `json:"original"`
	} `json:"urls"`
}

func GetAPIData(url string) (urlData []PixInfo, err error) {
	data, err := web.GetData(url)
	if err != nil {
		return nil, err
	}
	var result LoliconData
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, errors.New(result.Error)
	}
	return result.Data, nil
}

// TransListToIllust 将PixInfo列表转换为Illust列表
func TransListToIllust(data []PixInfo) (illustData []*Illust, err error) {
	illustData = make([]*Illust, 0, len(data))
	errList := make([]error, 0)
	for i, v := range data {
		illust, err := TransToIllust(v)
		if err != nil {
			errList = append(errList, fmt.Errorf("index: %d, error: %w", i, err))
			continue
		}
		illustData = append(illustData, illust)
	}
	if len(errList) > 0 {
		return illustData, errors.Join(errList...)
	}
	return illustData, nil
}

// TransToIllust 获取插画信息
func TransToIllust(data PixInfo) (i *Illust, err error) {
	// 解析返回插画信息
	i = &Illust{}
	i.Pid = data.Pid
	i.Title = data.Title
	i.UID = data.UID
	i.Author = data.Author
	i.Tags = data.Tags
	i.UploadDate = data.UploadDate

	// 修复：确保 P 字段有效
	index := data.P
	if index < 0 {
		index = 0
	}
	i.MaxPager = index + 1
	if data.R18 {
		i.AgeLimit = append(i.AgeLimit, index)
	}

	// 检查 URL 是否为空
	if data.Urls.Original == "" {
		return nil, fmt.Errorf("图片 URL 为空，PID: %d", data.Pid)
	}

	i.ImageUrls = map[int]string{
		index: data.Urls.Original,
	}
	return i, nil
}
