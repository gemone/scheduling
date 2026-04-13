package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/browser"
	"github.com/xuri/excelize/v2"
)

// ==================== 数据模型 ====================

type ShiftType string

const (
	DayShift   ShiftType = "白班"
	NightShift ShiftType = "夜班"
	OffDuty    ShiftType = "休"
)

type ShiftEntry struct {
	Date      string    `json:"date"`
	Person    string    `json:"person"`
	ShiftType ShiftType `json:"shift_type"`
}

type Person struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	MinTotal             int    `json:"min_total"`               // 强制最低排班总次数 (0=不强制)
	MaxTotal             int    `json:"max_total"`               // 月最大排班总次数
	MaxDay               int    `json:"max_day"`                 // 月最大白班次数
	MaxNight             int    `json:"max_night"`               // 月最大夜班次数
	DayShiftPos          int    `json:"day_shift_pos"`           // 1=可工作日白班 0=不可
	NightShiftPos        int    `json:"night_shift_pos"`         // 1=可工作日夜班 0=不可
	WeekendDayShiftPos   int    `json:"weekend_day_shift_pos"`   // 1=可周末白班 0=不可
	WeekendNightShiftPos int    `json:"weekend_night_shift_pos"` // 1=可周末夜班 0=不可
	HolidayDayShiftPos   int    `json:"holiday_day_shift_pos"`   // 1=可节假日白班 0=不可
	HolidayNightShiftPos int    `json:"holiday_night_shift_pos"` // 1=可节假日夜班 0=不可
}

type Vacation struct {
	PersonID string `json:"person_id"`
	Date     string `json:"date"` // YYYY-MM-DD
	Type     string `json:"type"` // "vacation" / "avoid"
}

type ScheduleRule struct {
	DayShiftPerDay    int `json:"day_shift_per_day"`   // 工作日白班人数
	NightShiftPerDay  int `json:"night_shift_per_day"` // 工作日夜班人数
	WeekendDayShift   int `json:"weekend_day_shift"`   // 周末白班人数
	WeekendNightShift int `json:"weekend_night_shift"` // 周末夜班人数
	HolidayDayShift   int `json:"holiday_day_shift"`   // 法定节假日白班人数
	HolidayNightShift int `json:"holiday_night_shift"` // 法定节假日夜班人数
}

type MonthData struct {
	People     []Person          `json:"people"`
	Vacations  []Vacation        `json:"vacations"`
	Rules      ScheduleRule      `json:"rules"`
	Schedule   []ShiftEntry      `json:"schedule"`
	PinnedDays []string          `json:"pinned_days"` // 固定的日期列表 (YYYY-MM-DD)
	DayTypes   map[string]string `json:"day_types"`   // 日期类型覆盖: "holiday" / "workday" (YYYY-MM-DD -> type)
	Year       int               `json:"year"`
	Month      int               `json:"month"`
}

// ==================== App ====================

type App struct {
	ctx     context.Context
	dataDir string
}

func NewApp() *App {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".shift-scheduler")
	os.MkdirAll(dataDir, 0755)
	return &App{dataDir: dataDir}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ==================== 全局人员 ====================

func (a *App) peoplePath() string {
	return filepath.Join(a.dataDir, "people.json")
}

func (a *App) rulesPath() string {
	return filepath.Join(a.dataDir, "rules.json")
}

func (a *App) LoadPeople() ([]Person, error) {
	path := a.peoplePath()
	b, err := os.ReadFile(path)
	if err != nil {
		return []Person{}, nil
	}
	var people []Person
	if err := json.Unmarshal(b, &people); err != nil {
		return nil, err
	}
	return people, nil
}

func (a *App) SavePeople(people []Person) error {
	b, err := json.MarshalIndent(people, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.peoplePath(), b, 0644)
}

func (a *App) LoadRules() (ScheduleRule, error) {
	path := a.rulesPath()
	b, err := os.ReadFile(path)
	if err != nil {
		return ScheduleRule{DayShiftPerDay: 1, NightShiftPerDay: 1, WeekendDayShift: 1, WeekendNightShift: 1, HolidayDayShift: 1, HolidayNightShift: 1}, nil
	}
	var rules ScheduleRule
	if err := json.Unmarshal(b, &rules); err != nil {
		return ScheduleRule{DayShiftPerDay: 1, NightShiftPerDay: 1, WeekendDayShift: 1, WeekendNightShift: 1, HolidayDayShift: 1, HolidayNightShift: 1}, nil
	}
	return rules, nil
}

func (a *App) SaveRules(rules ScheduleRule) error {
	b, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.rulesPath(), b, 0644)
}

// ==================== 数据持久化 ====================

func (a *App) dataPath(year, month int) string {
	return filepath.Join(a.dataDir, fmt.Sprintf("schedule_%04d_%02d.json", year, month))
}

func (a *App) SaveMonthData(data MonthData) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.dataPath(data.Year, data.Month), b, 0644)
}

func (a *App) LoadMonthData(year, month int) (*MonthData, error) {
	path := a.dataPath(year, month)
	b, err := os.ReadFile(path)
	if err != nil {
		// Return empty data (people will be filled from global)
		return &MonthData{
			People:    []Person{},
			Vacations: []Vacation{},
			Rules: ScheduleRule{
				DayShiftPerDay:    1,
				NightShiftPerDay:  1,
				WeekendDayShift:   1,
				WeekendNightShift: 1,
				HolidayDayShift:   1,
				HolidayNightShift: 1,
			},
			Schedule:   []ShiftEntry{},
			PinnedDays: []string{},
			DayTypes:   map[string]string{},
			Year:       year,
			Month:      month,
		}, nil
	}
	var data MonthData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// ==================== 排班算法 ====================

func (a *App) GenerateSchedule(data MonthData) (*MonthData, error) {
	daysInMonth := time.Date(data.Year, time.Month(data.Month)+1, 0, 0, 0, 0, 0, time.UTC).Day()

	// Build pinned set
	pinnedSet := make(map[string]bool)
	for _, d := range data.PinnedDays {
		pinnedSet[d] = true
	}

	// Keep pinned entries, collect counts from them
	schedule := make([]ShiftEntry, 0)
	pinnedCounts := make(map[string]*struct{ Total, Day, Night int })
	for _, p := range data.People {
		pinnedCounts[p.Name] = &struct{ Total, Day, Night int }{}
	}
	// Track who worked night shift on which date (from pinned)
	nightShiftDates := make(map[string]string) // personName -> date of night shift
	for _, entry := range data.Schedule {
		if pinnedSet[entry.Date] {
			schedule = append(schedule, entry)
			if pc, ok := pinnedCounts[entry.Person]; ok {
				pc.Total++
				switch entry.ShiftType {
				case DayShift:
					pc.Day++
				case NightShift:
					pc.Night++
					nightShiftDates[entry.Person] = entry.Date
				}
			}
		}
	}

	// Count per person (start from pinned counts)
	type PersonCount struct {
		Total int
		Day   int
		Night int
	}
	counts := make(map[string]*PersonCount)
	personMap := make(map[string]Person)
	for _, p := range data.People {
		pc := pinnedCounts[p.Name]
		counts[p.ID] = &PersonCount{Total: pc.Total, Day: pc.Day, Night: pc.Night}
		personMap[p.ID] = p
	}

	// Vacation set: personID -> date set
	vacationSet := make(map[string]map[string]bool)
	avoidSet := make(map[string]map[string]bool)
	for _, v := range data.Vacations {
		if v.Type == "vacation" {
			if vacationSet[v.PersonID] == nil {
				vacationSet[v.PersonID] = map[string]bool{}
			}
			vacationSet[v.PersonID][v.Date] = true
		} else {
			if avoidSet[v.PersonID] == nil {
				avoidSet[v.PersonID] = map[string]bool{}
			}
			avoidSet[v.PersonID][v.Date] = true
		}
	}

	// Helper: get previous day date string
	prevDate := func(y, m, day int) string {
		t := time.Date(y, time.Month(m), day, 0, 0, 0, 0, time.UTC)
		t = t.AddDate(0, 0, -1)
		return fmt.Sprintf("%04d-%02d-%02d", t.Year(), t.Month(), t.Day())
	}

	// Seed random for shuffle
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Build lookup for quick night-shift check: date -> set of person names on night shift
	// (populated as we go + from pinned)
	nightOnDate := make(map[string]map[string]bool)
	for _, entry := range data.Schedule {
		if pinnedSet[entry.Date] && entry.ShiftType == NightShift {
			if nightOnDate[entry.Date] == nil {
				nightOnDate[entry.Date] = map[string]bool{}
			}
			nightOnDate[entry.Date][entry.Person] = true
		}
	}

	// === 分轮次排班：先所有日期排第1人，再排第2人，依此类推 ===
	type DayInfo struct {
		Day           int
		DateStr       string
		EffectiveType string
		DayCount      int
		NightCount    int
		Available     []string
	}

	dayInfos := make([]DayInfo, 0, daysInMonth)
	for day := 1; day <= daysInMonth; day++ {
		dateStr := fmt.Sprintf("%04d-%02d-%02d", data.Year, data.Month, day)

		// Skip pinned days
		if pinnedSet[dateStr] {
			continue
		}

		// Collect available people, emit OffDuty for vacations
		var available []string
		for _, p := range data.People {
			if vacationSet[p.ID] != nil && vacationSet[p.ID][dateStr] {
				schedule = append(schedule, ShiftEntry{
					Date:      dateStr,
					Person:    p.Name,
					ShiftType: OffDuty,
				})
				continue
			}
			if avoidSet[p.ID] != nil && avoidSet[p.ID][dateStr] {
				continue
			}
			available = append(available, p.ID)
		}

		dayCount := data.Rules.DayShiftPerDay
		nightCount := data.Rules.NightShiftPerDay

		// Determine day type
		t := time.Date(data.Year, time.Month(data.Month), day, 0, 0, 0, 0, time.UTC)
		dow := t.Weekday()
		naturalWeekend := dow == time.Saturday || dow == time.Sunday

		effectiveType := "workday"
		if data.DayTypes != nil {
			if dt, ok := data.DayTypes[dateStr]; ok {
				effectiveType = dt
			} else if naturalWeekend {
				effectiveType = "weekend"
			}
		} else if naturalWeekend {
			effectiveType = "weekend"
		}

		switch effectiveType {
		case "holiday":
			if data.Rules.HolidayDayShift > 0 {
				dayCount = data.Rules.HolidayDayShift
			}
			if data.Rules.HolidayNightShift > 0 {
				nightCount = data.Rules.HolidayNightShift
			}
		case "weekend":
			if data.Rules.WeekendDayShift > 0 {
				dayCount = data.Rules.WeekendDayShift
			}
			if data.Rules.WeekendNightShift > 0 {
				nightCount = data.Rules.WeekendNightShift
			}
		}

		dayInfos = append(dayInfos, DayInfo{
			Day:           day,
			DateStr:       dateStr,
			EffectiveType: effectiveType,
			DayCount:      dayCount,
			NightCount:    nightCount,
			Available:     available,
		})
	}

	// 计算最大轮次数 = max(所有天的白班人数, 所有天的夜班人数)
	maxRounds := 0
	for _, di := range dayInfos {
		if di.DayCount > maxRounds {
			maxRounds = di.DayCount
		}
		if di.NightCount > maxRounds {
			maxRounds = di.NightCount
		}
	}

	dayAssignedCount := make(map[string]int)
	nightAssignedCount := make(map[string]int)
	allAssignedSet := make(map[string]map[string]bool)

	for _, di := range dayInfos {
		allAssignedSet[di.DateStr] = make(map[string]bool)
	}

	canDayShift := func(p Person, effectiveType string) bool {
		switch effectiveType {
		case "holiday":
			return p.HolidayDayShiftPos == 1
		case "weekend":
			return p.WeekendDayShiftPos == 1
		default:
			return p.DayShiftPos == 1
		}
	}
	canNightShift := func(p Person, effectiveType string) bool {
		switch effectiveType {
		case "holiday":
			return p.HolidayNightShiftPos == 1
		case "weekend":
			return p.WeekendNightShiftPos == 1
		default:
			return p.NightShiftPos == 1
		}
	}

	// 统计每个人在周末/节假日/工作日分别已排的白班和夜班次数（用于均衡分配）
	typeCountDay := make(map[string]map[string]int)   // pid -> effectiveType -> count
	typeCountNight := make(map[string]map[string]int) // pid -> effectiveType -> count
	for _, p := range data.People {
		typeCountDay[p.ID] = map[string]int{}
		typeCountNight[p.ID] = map[string]int{}
	}
	// 从 pinned 的排班中初始化 typeCount
	for _, entry := range data.Schedule {
		if !pinnedSet[entry.Date] {
			continue
		}
		pid := ""
		for _, p := range data.People {
			if p.Name == entry.Person {
				pid = p.ID
				break
			}
		}
		if pid == "" {
			continue
		}
		// 解析 pinned 条目的日期类型
		parts := strings.Split(entry.Date, "-")
		if len(parts) == 3 {
			var pDay int
			fmt.Sscanf(parts[2], "%d", &pDay)
			pt := time.Date(data.Year, time.Month(data.Month), pDay, 0, 0, 0, 0, time.UTC)
			pdow := pt.Weekday()
			pType := "workday"
			if data.DayTypes != nil {
				if dt, ok := data.DayTypes[entry.Date]; ok {
					pType = dt
				} else if pdow == time.Saturday || pdow == time.Sunday {
					pType = "weekend"
				}
			} else if pdow == time.Saturday || pdow == time.Sunday {
				pType = "weekend"
			}
			switch entry.ShiftType {
			case DayShift:
				typeCountDay[pid][pType]++
			case NightShift:
				typeCountNight[pid][pType]++
			}
		}
	}

	// 分轮次排班：第 round 轮为每天排第 round+1 个人
	for round := 0; round < maxRounds; round++ {
		// --- 白班轮次 ---
		for _, di := range dayInfos {
			if di.DayCount == 0 || dayAssignedCount[di.DateStr] >= di.DayCount {
				continue
			}

			// 昨夜夜班 → 今日不排白班
			prev := prevDate(data.Year, data.Month, di.Day)
			yesterdayNight := nightOnDate[prev]

			var dayCandidates []string
			for _, pid := range di.Available {
				if allAssignedSet[di.DateStr][pid] {
					continue
				}
				p := personMap[pid]
				if yesterdayNight != nil && yesterdayNight[p.Name] {
					continue
				}
				if !canDayShift(p, di.EffectiveType) {
					continue
				}
				dayCandidates = append(dayCandidates, pid)
			}

			if len(dayCandidates) == 0 {
				continue
			}

			sort.SliceStable(dayCandidates, func(i, j int) bool {
				pi := personMap[dayCandidates[i]]
				pj := personMap[dayCandidates[j]]
				ci := counts[dayCandidates[i]]
				cj := counts[dayCandidates[j]]
				// 优先：未达最低次数
				needI := pi.MinTotal > 0 && ci.Total < pi.MinTotal
				needJ := pj.MinTotal > 0 && cj.Total < pj.MinTotal
				if needI != needJ {
					return needI
				}
				// 优先：该类型日排班次数少（均衡周末/节假日分配）
				tcI := typeCountDay[dayCandidates[i]][di.EffectiveType]
				tcJ := typeCountDay[dayCandidates[j]][di.EffectiveType]
				if tcI != tcJ {
					return tcI < tcJ
				}
				if ci.Total != cj.Total {
					return ci.Total < cj.Total
				}
				if ci.Day != cj.Day {
					return ci.Day < cj.Day
				}
				return rng.Intn(2) == 0
			})

			assigned := false
			for _, pid := range dayCandidates {
				p := personMap[pid]
				pc := counts[pid]
				if pc.Total < p.MaxTotal && pc.Day < p.MaxDay {
					schedule = append(schedule, ShiftEntry{
						Date:      di.DateStr,
						Person:    p.Name,
						ShiftType: DayShift,
					})
					pc.Total++
					pc.Day++
					typeCountDay[pid][di.EffectiveType]++
					dayAssignedCount[di.DateStr]++
					allAssignedSet[di.DateStr][pid] = true
					assigned = true
					break
				}
			}

			// 保底：round 0 且需求>0但无人通过正常上限检查时，放宽限制强排第一个候选人
			if !assigned && round == 0 && dayAssignedCount[di.DateStr] == 0 {
				pid := dayCandidates[0]
				p := personMap[pid]
				pc := counts[pid]
				schedule = append(schedule, ShiftEntry{
					Date:      di.DateStr,
					Person:    p.Name,
					ShiftType: DayShift,
				})
				pc.Total++
				pc.Day++
				typeCountDay[pid][di.EffectiveType]++
				dayAssignedCount[di.DateStr]++
				allAssignedSet[di.DateStr][pid] = true
			}
		}

		// --- 夜班轮次 ---
		for _, di := range dayInfos {
			if di.NightCount == 0 || nightAssignedCount[di.DateStr] >= di.NightCount {
				continue
			}

			var nightCandidates []string
			for _, pid := range di.Available {
				if allAssignedSet[di.DateStr][pid] {
					continue
				}
				if !canNightShift(personMap[pid], di.EffectiveType) {
					continue
				}
				nightCandidates = append(nightCandidates, pid)
			}

			if len(nightCandidates) == 0 {
				continue
			}

			sort.SliceStable(nightCandidates, func(i, j int) bool {
				pi := personMap[nightCandidates[i]]
				pj := personMap[nightCandidates[j]]
				ci := counts[nightCandidates[i]]
				cj := counts[nightCandidates[j]]
				needI := pi.MinTotal > 0 && ci.Total < pi.MinTotal
				needJ := pj.MinTotal > 0 && cj.Total < pj.MinTotal
				if needI != needJ {
					return needI
				}
				// 优先：该类型日夜班次数少（均衡周末/节假日分配）
				tcI := typeCountNight[nightCandidates[i]][di.EffectiveType]
				tcJ := typeCountNight[nightCandidates[j]][di.EffectiveType]
				if tcI != tcJ {
					return tcI < tcJ
				}
				if ci.Total != cj.Total {
					return ci.Total < cj.Total
				}
				if ci.Night != cj.Night {
					return ci.Night < cj.Night
				}
				return rng.Intn(2) == 0
			})

			assigned := false
			for _, pid := range nightCandidates {
				p := personMap[pid]
				pc := counts[pid]
				if pc.Total < p.MaxTotal && pc.Night < p.MaxNight {
					schedule = append(schedule, ShiftEntry{
						Date:      di.DateStr,
						Person:    p.Name,
						ShiftType: NightShift,
					})
					pc.Total++
					pc.Night++
					typeCountNight[pid][di.EffectiveType]++
					nightAssignedCount[di.DateStr]++
					allAssignedSet[di.DateStr][pid] = true
					if nightOnDate[di.DateStr] == nil {
						nightOnDate[di.DateStr] = map[string]bool{}
					}
					nightOnDate[di.DateStr][p.Name] = true
					assigned = true
					break
				}
			}

			// 保底：round 0 且需求>0但无人通过正常上限检查时，放宽限制强排第一个候选人
			if !assigned && round == 0 && nightAssignedCount[di.DateStr] == 0 {
				pid := nightCandidates[0]
				p := personMap[pid]
				pc := counts[pid]
				schedule = append(schedule, ShiftEntry{
					Date:      di.DateStr,
					Person:    p.Name,
					ShiftType: NightShift,
				})
				pc.Total++
				pc.Night++
				typeCountNight[pid][di.EffectiveType]++
				nightAssignedCount[di.DateStr]++
				allAssignedSet[di.DateStr][pid] = true
				if nightOnDate[di.DateStr] == nil {
					nightOnDate[di.DateStr] = map[string]bool{}
				}
				nightOnDate[di.DateStr][p.Name] = true
			}
		}
	}

	// Sort schedule by date, then shift order (白班 before 夜班 before 休)
	shiftOrder := map[ShiftType]int{DayShift: 0, NightShift: 1, OffDuty: 2}
	sort.SliceStable(schedule, func(i, j int) bool {
		if schedule[i].Date != schedule[j].Date {
			return schedule[i].Date < schedule[j].Date
		}
		return shiftOrder[schedule[i].ShiftType] < shiftOrder[schedule[j].ShiftType]
	})

	data.Schedule = schedule
	return &data, nil
}

// ==================== 导出 ====================

func (a *App) ExportXLSX(data MonthData) (string, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "排班表"
	f.SetSheetName("Sheet1", sheet)

	daysInMonth := time.Date(data.Year, time.Month(data.Month)+1, 0, 0, 0, 0, 0, time.UTC).Day()

	// Build lookup: date -> shift_type -> []personName
	typeByDate := make(map[string]map[string][]string)
	for _, entry := range data.Schedule {
		if typeByDate[entry.Date] == nil {
			typeByDate[entry.Date] = map[string][]string{}
		}
		st := string(entry.ShiftType)
		typeByDate[entry.Date][st] = append(typeByDate[entry.Date][st], entry.Person)
	}

	weekdayNames := []string{"日", "一", "二", "三", "四", "五", "六"}
	shiftOrder := []ShiftType{DayShift, NightShift, OffDuty}
	shiftLabel := map[ShiftType]string{
		DayShift:   "白班",
		NightShift: "夜班",
		OffDuty:    "休假",
	}

	// Styles
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 16},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "FFFFFF"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#4f46e5"}, Pattern: 1},
	})
	dayStyle, _ := f.NewStyle(&excelize.Style{
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#fef3c7"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	nightStyle, _ := f.NewStyle(&excelize.Style{
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#e0e7ff"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	offStyle, _ := f.NewStyle(&excelize.Style{
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#f3f4f6"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	shiftStyles := map[ShiftType]int{
		DayShift:   dayStyle,
		NightShift: nightStyle,
		OffDuty:    offStyle,
	}
	labelStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	// Title row
	titleText := fmt.Sprintf("%d年%d月 排班表", data.Year, data.Month)
	f.SetCellValue(sheet, "A1", titleText)
	f.SetCellStyle(sheet, "A1", "A1", titleStyle)

	// Header row (row 3): blank | 1日(一) | 2日(二) | ...
	headerRow := 3
	f.SetCellValue(sheet, fmt.Sprintf("A%d", headerRow), "")
	for day := 1; day <= daysInMonth; day++ {
		col, _ := excelize.CoordinatesToCellName(day+1, headerRow)
		dow := time.Date(data.Year, time.Month(data.Month), day, 0, 0, 0, 0, time.UTC).Weekday()
		f.SetCellValue(sheet, col, fmt.Sprintf("%d日(%s)", day, weekdayNames[dow]))
		f.SetCellStyle(sheet, col, col, headerStyle)
	}
	// Style the label column header too
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", headerRow), fmt.Sprintf("A%d", headerRow), headerStyle)

	// Data rows: one per shift type
	for rowIdx, st := range shiftOrder {
		row := headerRow + 1 + rowIdx
		label := shiftLabel[st]
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), label)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), labelStyle)
		for day := 1; day <= daysInMonth; day++ {
			dateStr := fmt.Sprintf("%04d-%02d-%02d", data.Year, data.Month, day)
			names := typeByDate[dateStr][string(st)]
			cell := ""
			if len(names) > 0 {
				cell = strings.Join(names, "\n")
			}
			col, _ := excelize.CoordinatesToCellName(day+1, row)
			f.SetCellValue(sheet, col, cell)
			style, ok := shiftStyles[st]
			if ok {
				f.SetCellStyle(sheet, col, col, style)
			}
		}
	}

	// Summary row
	summaryRow := headerRow + 1 + len(shiftOrder) + 1
	f.SetCellValue(sheet, fmt.Sprintf("A%d", summaryRow), "统计")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", summaryRow), fmt.Sprintf("A%d", summaryRow), labelStyle)
	for day := 1; day <= daysInMonth; day++ {
		dateStr := fmt.Sprintf("%04d-%02d-%02d", data.Year, data.Month, day)
		dayN := len(typeByDate[dateStr][string(DayShift)])
		nightN := len(typeByDate[dateStr][string(NightShift)])
		offN := len(typeByDate[dateStr][string(OffDuty)])
		col, _ := excelize.CoordinatesToCellName(day+1, summaryRow)
		f.SetCellValue(sheet, col, fmt.Sprintf("白%d 夜%d 休%d", dayN, nightN, offN))
	}

	// Per-person summary
	peepRow := summaryRow + 2
	f.SetCellValue(sheet, fmt.Sprintf("A%d", peepRow), "人员统计")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", peepRow), fmt.Sprintf("A%d", peepRow), titleStyle)
	peepRow++
	f.SetCellValue(sheet, fmt.Sprintf("A%d", peepRow), "姓名")
	f.SetCellValue(sheet, fmt.Sprintf("B%d", peepRow), "白班")
	f.SetCellValue(sheet, fmt.Sprintf("C%d", peepRow), "夜班")
	f.SetCellValue(sheet, fmt.Sprintf("D%d", peepRow), "总计")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", peepRow), fmt.Sprintf("D%d", peepRow), headerStyle)
	peepRow++

	for _, p := range data.People {
		dayCount := 0
		nightCount := 0
		for _, entry := range data.Schedule {
			if entry.Person == p.Name {
				switch entry.ShiftType {
				case DayShift:
					dayCount++
				case NightShift:
					nightCount++
				}
			}
		}
		f.SetCellValue(sheet, fmt.Sprintf("A%d", peepRow), p.Name)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", peepRow), dayCount)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", peepRow), nightCount)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", peepRow), dayCount+nightCount)
		peepRow++
	}

	// Column widths
	f.SetColWidth(sheet, "A", "A", 10)
	for day := 1; day <= daysInMonth; day++ {
		colName, _ := excelize.ColumnNumberToName(day + 1)
		f.SetColWidth(sheet, colName, colName, 12)
	}

	// Save
	home, _ := os.UserHomeDir()
	downloadDir := filepath.Join(home, "Downloads")
	os.MkdirAll(downloadDir, 0755)
	filename := fmt.Sprintf("排班表_%d年%d月.xlsx", data.Year, data.Month)
	filePath := filepath.Join(downloadDir, filename)
	if err := f.SaveAs(filePath); err != nil {
		return "", err
	}
	return filePath, nil
}

func (a *App) SaveExportFile(content, filename string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	downloadDir := filepath.Join(home, "Downloads")
	os.MkdirAll(downloadDir, 0755)
	path := filepath.Join(downloadDir, filename)
	return os.WriteFile(path, []byte(content), 0644)
}

func (a *App) OpenFile(path string) error {
	return browser.OpenFile(path)
}

// ==================== 人员 XLSX 导入导出 ====================

func (a *App) ExportPeopleXLSX(people []Person) (string, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "人员列表"
	f.SetSheetName("Sheet1", sheet)

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "FFFFFF"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#4f46e5"}, Pattern: 1},
	})
	centerStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	headers := []string{"姓名", "强制排满次数", "月最大总班次", "月最大白班", "月最大夜班",
		"工作日白班", "工作日夜班", "周末白班", "周末夜班", "节假日白班", "节假日夜班"}
	for i, h := range headers {
		col, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, col, h)
		f.SetCellStyle(sheet, col, col, headerStyle)
	}

	for row, p := range people {
		r := row + 2
		vals := []interface{}{
			p.Name, p.MinTotal, p.MaxTotal, p.MaxDay, p.MaxNight,
			boolStr(p.DayShiftPos), boolStr(p.NightShiftPos),
			boolStr(p.WeekendDayShiftPos), boolStr(p.WeekendNightShiftPos),
			boolStr(p.HolidayDayShiftPos), boolStr(p.HolidayNightShiftPos),
		}
		for i, v := range vals {
			col, _ := excelize.CoordinatesToCellName(i+1, r)
			f.SetCellValue(sheet, col, v)
			f.SetCellStyle(sheet, col, col, centerStyle)
		}
	}

	// Column widths
	widths := []float64{12, 14, 14, 12, 12, 10, 10, 10, 10, 10, 10}
	for i, w := range widths {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheet, colName, colName, w)
	}

	home, _ := os.UserHomeDir()
	downloadDir := filepath.Join(home, "Downloads")
	os.MkdirAll(downloadDir, 0755)
	filePath := filepath.Join(downloadDir, "人员列表.xlsx")
	if err := f.SaveAs(filePath); err != nil {
		return "", err
	}
	return filePath, nil
}

func (a *App) ImportPeopleXLSX() ([]Person, error) {
	// Use runtime file dialog
	return nil, fmt.Errorf("use frontend dialog")
}

func (a *App) ParsePeopleXLSX(filePath string) ([]Person, error) {
	return nil, fmt.Errorf("use ParsePeopleXLSXBase64")
}

func (a *App) ParsePeopleXLSXBase64(b64 string) ([]Person, error) {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64解码失败: %v", err)
	}

	tmpFile, err := os.CreateTemp("", "import_*.xlsx")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Write(data)
	tmpFile.Close()

	return parsePeopleXLSXFromFile(tmpFile.Name())
}

func parsePeopleXLSXFromFile(filePath string) ([]Person, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法打开文件: %v", err)
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("无法读取工作表: %v", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("文件为空或没有数据行")
	}

	// Parse header to find column indices
	header := rows[0]
	colMap := make(map[string]int)
	for i, h := range header {
		colMap[strings.TrimSpace(h)] = i
	}

	var people []Person
	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]
		name := getCol(row, colMap, "姓名")
		if name == "" {
			continue
		}
		p := Person{
			ID:                   fmt.Sprintf("p_import_%d_%d", time.Now().UnixNano(), rowIdx),
			Name:                 name,
			MinTotal:             getColInt(row, colMap, "强制排满次数"),
			MaxTotal:             getColIntDef(row, colMap, "月最大总班次", 22),
			MaxDay:               getColIntDef(row, colMap, "月最大白班", 15),
			MaxNight:             getColIntDef(row, colMap, "月最大夜班", 10),
			DayShiftPos:          parseBool(getCol(row, colMap, "工作日白班")),
			NightShiftPos:        parseBool(getCol(row, colMap, "工作日夜班")),
			WeekendDayShiftPos:   parseBool(getCol(row, colMap, "周末白班")),
			WeekendNightShiftPos: parseBool(getCol(row, colMap, "周末夜班")),
			HolidayDayShiftPos:   parseBool(getCol(row, colMap, "节假日白班")),
			HolidayNightShiftPos: parseBool(getCol(row, colMap, "节假日夜班")),
		}
		people = append(people, p)
	}
	return people, nil
}

func (a *App) DownloadPeopleTemplate() (string, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "人员导入模板"
	f.SetSheetName("Sheet1", sheet)

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "FFFFFF"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#4f46e5"}, Pattern: 1},
	})
	hintStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "888888", Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	headers := []string{"姓名", "强制排满次数", "月最大总班次", "月最大白班", "月最大夜班",
		"工作日白班", "工作日夜班", "周末白班", "周末夜班", "节假日白班", "节假日夜班"}
	hints := []string{"张三", "0", "22", "15", "10", "是", "是", "是", "否", "是", "否"}

	for i, h := range headers {
		col, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, col, h)
		f.SetCellStyle(sheet, col, col, headerStyle)
	}
	for i, h := range hints {
		col, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue(sheet, col, h)
		f.SetCellStyle(sheet, col, col, hintStyle)
	}

	// Instructions row
	f.SetCellValue(sheet, "A4", "说明：")
	f.SetCellValue(sheet, "A5", "1. 姓名：必填")
	f.SetCellValue(sheet, "A6", "2. 强制排满次数：0=不强制，填数字表示最少排满多少次")
	f.SetCellValue(sheet, "A7", "3. 月最大总班次/白班/夜班：限制每月最多排班次数")
	f.SetCellValue(sheet, "A8", "4. 班次类型：填\"是\"或\"否\"，也可以填 1 或 0")

	widths := []float64{12, 14, 14, 12, 12, 10, 10, 10, 10, 10, 10}
	for i, w := range widths {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheet, colName, colName, w)
	}

	home, _ := os.UserHomeDir()
	downloadDir := filepath.Join(home, "Downloads")
	os.MkdirAll(downloadDir, 0755)
	filePath := filepath.Join(downloadDir, "人员导入模板.xlsx")
	if err := f.SaveAs(filePath); err != nil {
		return "", err
	}
	return filePath, nil
}

func boolStr(v int) string {
	if v == 1 {
		return "是"
	}
	return "否"
}

func parseBool(s string) int {
	s = strings.TrimSpace(s)
	if s == "是" || s == "1" || strings.EqualFold(s, "yes") || strings.EqualFold(s, "true") {
		return 1
	}
	return 0
}

func getCol(row []string, colMap map[string]int, key string) string {
	idx, ok := colMap[key]
	if !ok || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func getColInt(row []string, colMap map[string]int, key string) int {
	s := getCol(row, colMap, key)
	if s == "" {
		return 0
	}
	n := 0
	fmt.Sscanf(s, "%d", &n)
	return n
}

func getColIntDef(row []string, colMap map[string]int, key string, def int) int {
	s := getCol(row, colMap, key)
	if s == "" {
		return def
	}
	n := 0
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return def
	}
	return n
}

// ==================== 手动编辑 ====================

// UpdateShiftEntry updates or adds a single shift entry (for manual editing)
func (a *App) UpdateShiftEntry(data MonthData, date string, personName string, shiftType string) (*MonthData, error) {
	// Remove existing entry for this date+person
	newSchedule := make([]ShiftEntry, 0)
	for _, entry := range data.Schedule {
		if !(entry.Date == date && entry.Person == personName) {
			newSchedule = append(newSchedule, entry)
		}
	}

	// Add new entry if not empty
	if shiftType != "" && shiftType != "-" {
		newSchedule = append(newSchedule, ShiftEntry{
			Date:      date,
			Person:    personName,
			ShiftType: ShiftType(shiftType),
		})
	}

	data.Schedule = newSchedule
	return &data, nil
}
