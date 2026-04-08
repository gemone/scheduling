# Scheduling

排班表桌面应用，基于 Wails + Vue 3 构建。

## 功能

- 👥 人员管理（添加/编辑/删除，支持强制排满次数、最大班次限制）
- 📅 日历视图（月视图，拖拽调整排班）
- 🔄 智能排班算法（均衡分配、夜班→次日白班约束、随机打乱）
- 📌 固定/解固日期（固定后重新排班不受影响）
- 🏖️ 休假/规避管理
- 📊 排班统计（总览每人白班/夜班/总次数）
- 📤 导出 CSV（日历网格格式）

## 技术栈

- **后端**: Go 1.26 + Wails v2.11
- **前端**: Vue 3 + TypeScript + Vite
- **数据存储**: JSON 文件（`~/.shift-scheduler/`）

## 开发

```bash
# 安装依赖
cd frontend && npm install

# 开发模式
wails dev

# 构建
wails build
```

## 构建产物

`build/bin/shift-scheduler.app`（macOS）
