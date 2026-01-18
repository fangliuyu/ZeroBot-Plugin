// Package handou 猜成语
package handou

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/FloatTech/floatbox/file"
	"github.com/sirupsen/logrus"
)

type UserHabits struct {
	mu          sync.RWMutex
	habits      map[string]int // 单字频率
	bigrams     map[string]int // 二元组频率
	idioms      map[string]int // 成语出现频率
	totalWords  int            // 总字数
	totalIdioms int            // 总成语数
	lastUpdate  time.Time      // 最后更新时间
}

var userHabits *UserHabits

// 初始化用户习惯
func initUserHabits() error {
	userHabits = &UserHabits{
		habits:  make(map[string]int),
		bigrams: make(map[string]int),
		idioms:  make(map[string]int),
	}

	if file.IsNotExist(userHabitsFile) {
		f, err := os.Create(userHabitsFile)
		if err != nil {
			return fmt.Errorf("创建用户习惯库时发生错误: %v", err)
		}
		_ = f.Close()
		return saveHabits()
	}

	// 读取现有习惯数据
	habitsFile, err := os.ReadFile(userHabitsFile)
	if err != nil {
		return fmt.Errorf("读取用户习惯库时发生错误: %v", err)
	}

	var savedData struct {
		Habits      map[string]int `json:"habits"`
		Bigrams     map[string]int `json:"bigrams"`
		Idioms      map[string]int `json:"idioms"`
		TotalWords  int            `json:"total_words"`
		TotalIdioms int            `json:"total_idioms"`
		LastUpdate  time.Time      `json:"last_update"`
	}

	if err := json.Unmarshal(habitsFile, &savedData); err != nil {
		// 如果是旧格式，尝试兼容
		var oldHabits map[string]int
		if err := json.Unmarshal(habitsFile, &oldHabits); err == nil {
			savedData.Habits = oldHabits
			// 从旧数据重新计算统计信息
			for _, count := range oldHabits {
				savedData.TotalWords += count
			}
		} else {
			return fmt.Errorf("解析用户习惯库时发生错误: %v", err)
		}
	}

	userHabits.mu.Lock()
	defer userHabits.mu.Unlock()

	userHabits.habits = savedData.Habits
	userHabits.bigrams = savedData.Bigrams
	userHabits.idioms = savedData.Idioms
	userHabits.totalWords = savedData.TotalWords
	userHabits.totalIdioms = savedData.TotalIdioms
	userHabits.lastUpdate = savedData.LastUpdate

	return nil
}

// 保存习惯数据
func saveHabits() error {
	userHabits.mu.RLock()
	defer userHabits.mu.RUnlock()

	data := struct {
		Habits      map[string]int `json:"habits"`
		Bigrams     map[string]int `json:"bigrams"`
		Idioms      map[string]int `json:"idioms"`
		TotalWords  int            `json:"total_words"`
		TotalIdioms int            `json:"total_idioms"`
		LastUpdate  time.Time      `json:"last_update"`
	}{
		Habits:      userHabits.habits,
		Bigrams:     userHabits.bigrams,
		Idioms:      userHabits.idioms,
		TotalWords:  userHabits.totalWords,
		TotalIdioms: userHabits.totalIdioms,
		LastUpdate:  time.Now(),
	}

	f, err := os.Create(userHabitsFile)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// 更新用户习惯（累加频率）
func updateHabits(input string) error {
	if userHabits == nil {
		if err := initUserHabits(); err != nil {
			return err
		}
	}

	userHabits.mu.Lock()
	defer userHabits.mu.Unlock()

	// 统计单字和二元组
	chars := []rune(input)
	userHabits.totalWords += len(chars)

	// 更新单字频率
	for _, char := range chars {
		charStr := string(char)
		userHabits.habits[charStr]++
	}

	// 仅当成语存在时，更新成语相关频率
	if slices.Contains(habitsIdiomKeys, input) {
		// 更新二元组频率（N=2的gram）
		for i := 0; i < len(chars)-1; i++ {
			bigram := string(chars[i]) + string(chars[i+1])
			userHabits.bigrams[bigram]++
		}
		// 更新成语频率
		userHabits.idioms[input]++
		userHabits.totalIdioms++
	}

	// 异步保存到文件
	go func() {
		if err := saveHabits(); err != nil {
			logrus.Warn("保存用户习惯时发生错误: ", err)
		}
	}()

	return nil
}

// 计算成语的优先级分数
func calculatePriorityScore(idiom string) float64 {
	if userHabits == nil || userHabits.totalWords == 0 {
		return 0
	}

	userHabits.mu.RLock()
	defer userHabits.mu.RUnlock()

	chars := []rune(idiom)
	score := 0.0

	// 1. 基于单字频率的分数
	for _, char := range chars {
		charStr := string(char)
		if count, exists := userHabits.habits[charStr]; exists {
			// 使用TF-IDF思想：频率越高，权重越高，但通过总字数归一化
			tf := float64(count) / float64(userHabits.totalWords)
			score += tf * 100
		}
	}

	// 2. 基于二元组频率的分数（词序的重要性）
	for i := 0; i < len(chars)-1; i++ {
		bigram := string(chars[i]) + string(chars[i+1])
		if count, exists := userHabits.bigrams[bigram]; exists {
			tf := float64(count) / float64(userHabits.totalWords)
			score += tf * 150 // 二元组比单字更重要
		}
	}

	// 3. 基于成语本身的频率（降低常见成语的优先级，增加多样性）
	if idiomCount, exists := userHabits.idioms[idiom]; exists {
		// 出现次数越多，优先级越低（避免总是出现相同的成语）
		penalty := float64(idiomCount) / float64(userHabits.totalIdioms) * 200
		score -= penalty
	}

	// 4. 考虑成语长度（适当增加长成语的权重）
	score += float64(len(chars)) * 10

	return score
}

// 优先抽取数据
func prioritizeData(data []string) []string {
	if len(data) == 0 {
		return data
	}

	// 计算每个成语的优先级分数
	idiomScores := make([]struct {
		idiom string
		score float64
	}, len(data))

	for i, idiom := range data {
		idiomScores[i] = struct {
			idiom string
			score float64
		}{
			idiom: idiom,
			score: calculatePriorityScore(idiom),
		}
	}

	// 按分数排序（从高到低）
	slices.SortFunc(idiomScores, func(a, b struct {
		idiom string
		score float64
	}) int {
		if a.score > b.score {
			return -1
		} else if a.score < b.score {
			return 1
		}
		return 0
	})

	// 选择前10个作为优先数据
	limit := min(len(idiomScores), 10)

	prioritized := make([]string, limit)
	for i := range limit {
		prioritized[i] = idiomScores[i].idiom
	}

	return prioritized
}

// 获取热门汉字（用于调试或展示）
func getTopCharacters(limit int) []string {
	if userHabits == nil {
		return nil
	}

	userHabits.mu.RLock()
	defer userHabits.mu.RUnlock()

	type charFreq struct {
		char  string
		count int
	}

	chars := make([]charFreq, 0, len(userHabits.habits))
	for char, count := range userHabits.habits {
		chars = append(chars, charFreq{char, count})
	}

	slices.SortFunc(chars, func(a, b charFreq) int {
		return b.count - a.count
	})

	if len(chars) > limit {
		chars = chars[:limit]
	}

	result := make([]string, len(chars))
	for i, cf := range chars {
		result[i] = fmt.Sprintf("%s:%d", cf.char, cf.count)
	}

	return result
}

// 获取热门成语（用于调试或展示）
func getTopIdioms(limit int) []string {
	if userHabits == nil {
		return nil
	}

	userHabits.mu.RLock()
	defer userHabits.mu.RUnlock()

	type idiomFreq struct {
		idiom string
		count int
	}

	idioms := make([]idiomFreq, 0, len(userHabits.idioms))
	for char, count := range userHabits.idioms {
		idioms = append(idioms, idiomFreq{char, count})
	}

	slices.SortFunc(idioms, func(a, b idiomFreq) int {
		return b.count - a.count
	})

	if len(idioms) > limit {
		idioms = idioms[:limit]
	}

	result := make([]string, len(idioms))
	for i, cf := range idioms {
		result[i] = fmt.Sprintf("%s:%d", cf.idiom, cf.count)
	}

	return result
}
