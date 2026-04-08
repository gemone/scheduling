package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
	ID            string `json:"id"`
	Name          string `json:"name"`
	MinTotal      int    `json:"min_total"`      // 强制最低排班总次数 (0=不强制)
	MaxTotal      int    `json:"max_total"`      // 月最大排班总次数
	MaxDay        int    `json:"max_day"`        // 月最大白班次数
	MaxNight      int    `json:"max_night"`      // 月最大夜班次数
	DayShiftPos   int    `json:"day_shift_pos"`   // 1=可白班 0=不可
	NightShiftPos int    `json:"night_shift_pos"` // 1=可夜班 0=不可
}

type Vacation struct {
	PersonID string `json:"person_id"`
	Date     string `json:"date"` // YYYY-MM-DD
	Type     string `json:"type"` // "vacation" / "avoid"
}

type ScheduleRule struct {
	DayShiftPerDay   int `json:"day_shift_per_day"`   // 每天白班人数
	NightShiftPerDay int `json:"night_shift_per_day"` // 每天夜班人数
}

type MonthData struct {
	People     []Person     `json:"people"`
	Vacations  []Vacation   `json:"vacations"`
	Rules      ScheduleRule `json:"rules"`
	Schedule   []ShiftEntry `json:"schedule"`
	PinnedDays []string     `json:"pinned_days"` // 固定的日期列表 (YYYY-MM-DD)
	Year       int          `json:"year"`
	Month      int          `json:"month"`
}

// ==================== App ====================

type App struct {
	ctx      context.Context
	dataDir  string
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
			People:     []Person{},
			Vacations:  []Vacation{},
			Rules:      ScheduleRule{DayShiftPerDay: 1, NightShiftPerDay: 1},
			Schedule:   []ShiftEntry{},
			PinnedDays: []string{},
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
	
	// For each day
	for day := 1; day <= daysInMonth; day++ {
		dateStr := fmt.Sprintf("%04d-%02d-%02d", data.Year, data.Month, day)
		
		// Skip pinned days
		if pinnedSet[dateStr] {
			continue
		}
		
		// Who worked night shift yesterday?
		prev := prevDate(data.Year, data.Month, day)
		yesterdayNight := nightOnDate[prev] // person names who did night shift yesterday
		
		// Collect available people for this day
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
		dayAssigned := 0
		nightAssigned := 0
		assigned := make(map[string]bool)

		// === Assign day shifts ===
		// Filter: exclude people who did night shift yesterday
		var dayCandidates []string
		for _, pid := range available {
			p := personMap[pid]
			if yesterdayNight != nil && yesterdayNight[p.Name] {
				continue // 昨晚夜班，今天不能白班
			}
			dayCandidates = append(dayCandidates, pid)
		}
		
		// Sort: prioritize people below min_total, then fewer total, then fewer day shifts, random tie-break
		sort.SliceStable(dayCandidates, func(i, j int) bool {
			pi := personMap[dayCandidates[i]]
			pj := personMap[dayCandidates[j]]
			ci := counts[dayCandidates[i]]
			cj := counts[dayCandidates[j]]
			needI := pi.MinTotal > 0 && ci.Total < pi.MinTotal
			needJ := pj.MinTotal > 0 && cj.Total < pj.MinTotal
			if needI != needJ {
				return needI
			}
			if ci.Total != cj.Total {
				return ci.Total < cj.Total
			}
			if ci.Day != cj.Day {
				return ci.Day < cj.Day
			}
			return rng.Intn(2) == 0
		})
		
		for _, pid := range dayCandidates {
			if dayAssigned >= dayCount {
				break
			}
			p := personMap[pid]
			pc := counts[pid]
			
			if p.DayShiftPos == 1 && pc.Total < p.MaxTotal && pc.Day < p.MaxDay {
				schedule = append(schedule, ShiftEntry{
					Date:      dateStr,
					Person:    p.Name,
					ShiftType: DayShift,
				})
				pc.Total++
				pc.Day++
				dayAssigned++
				assigned[pid] = true
			}
		}
		
		// === Assign night shifts ===
		nightCandidates := make([]string, 0, len(available))
		for _, pid := range available {
			if !assigned[pid] {
				nightCandidates = append(nightCandidates, pid)
			}
		}
		
		// Sort: prioritize people below min_total, then fewer total, then fewer night, random tie-break
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
			if ci.Total != cj.Total {
				return ci.Total < cj.Total
			}
			if ci.Night != cj.Night {
				return ci.Night < cj.Night
			}
			return rng.Intn(2) == 0
		})
		
		for _, pid := range nightCandidates {
			if nightAssigned >= nightCount {
				break
			}
			p := personMap[pid]
			pc := counts[pid]
			
			if p.NightShiftPos == 1 && pc.Total < p.MaxTotal && pc.Night < p.MaxNight {
				schedule = append(schedule, ShiftEntry{
					Date:      dateStr,
					Person:    p.Name,
					ShiftType: NightShift,
				})
				pc.Total++
				pc.Night++
				nightAssigned++
				// Track night shift for next day's constraint
				if nightOnDate[dateStr] == nil {
					nightOnDate[dateStr] = map[string]bool{}
				}
				nightOnDate[dateStr][p.Name] = true
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

func (a *App) ExportCSV(data MonthData) (string, error) {
	var sb strings.Builder

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
	shiftOrder := []string{string(DayShift), string(NightShift), string(OffDuty)}
	shiftLabel := map[string]string{
		string(DayShift):   "白班",
		string(NightShift): "夜班",
		string(OffDuty):    "休假",
	}

	// Title
	sb.WriteString(fmt.Sprintf("%d年%d月 排班表\n\n", data.Year, data.Month))

	// Header row: blank | 1日(一) | 2日(二) | ...
	sb.WriteString("")
	for day := 1; day <= daysInMonth; day++ {
		dow := time.Date(data.Year, time.Month(data.Month), day, 0, 0, 0, 0, time.UTC).Weekday()
		sb.WriteString(fmt.Sprintf("\t%d日(%s)", day, weekdayNames[dow]))
	}
	sb.WriteString("\n")

	// One row per shift type, cells contain person names
	for _, st := range shiftOrder {
		label := shiftLabel[st]
		sb.WriteString(label)
		for day := 1; day <= daysInMonth; day++ {
			dateStr := fmt.Sprintf("%04d-%02d-%02d", data.Year, data.Month, day)
			names := typeByDate[dateStr][st]
			cell := ""
			if len(names) > 0 {
				cell = strings.Join(names, "、")
			}
			sb.WriteString("\t" + cell)
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
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
