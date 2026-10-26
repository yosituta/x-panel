package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"x-ui/config"
	"x-ui/database"
	"x-ui/logger"
	"x-ui/web/service"
)

// runBackupCommand 提供终端下的快照/还原能力：
//
//	x-ui backup                  立即生成一份快照
//	x-ui backup -list            列出已有快照
//	x-ui backup -restore <文件>  从快照还原（面板需先停止）
//	x-ui backup -keep 20         生成时保留的份数（默认 10）
//
// 【开源增强】面板崩了也能在 SSH 里一条命令救回来，不用重装系统。
func runBackupCommand(args []string) {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	list := fs.Bool("list", false, "列出已有快照")
	restore := fs.String("restore", "", "从指定快照文件还原")
	keep := fs.Int("keep", 10, "生成快照时保留的份数")
	_ = fs.Parse(args)

	var svc service.BackupService

	if *restore != "" {
		fmt.Println("⚠️  还原会覆盖当前数据库，请确认面板已停止（systemctl stop x-panel）")
		if err := svc.Restore(*restore); err != nil {
			fmt.Println("还原失败:", err)
			os.Exit(1)
		}
		fmt.Println("✅ 还原完成，请执行：systemctl start x-panel")
		return
	}

	if *list {
		items, err := svc.List()
		if err != nil {
			fmt.Println("读取快照列表失败:", err)
			os.Exit(1)
		}
		if len(items) == 0 {
			fmt.Println("还没有任何快照，执行 `x-ui backup` 生成第一份")
			return
		}
		fmt.Printf("%-28s %10s  %s\n", "快照文件", "大小", "时间")
		for _, it := range items {
			fmt.Printf("%-28s %9.1fK  %s\n", it.Name, float64(it.Size)/1024,
				time.UnixMilli(it.Time).Format("2006-01-02 15:04:05"))
		}
		return
	}

	// 生成快照需要读取数据库
	logger.InitLogger(0)
	if err := database.InitDB(config.GetDBPath()); err != nil {
		fmt.Println("打开数据库失败:", err)
		os.Exit(1)
	}
	snap, err := svc.Create(*keep)
	if err != nil {
		fmt.Println("生成快照失败:", err)
		os.Exit(1)
	}
	fmt.Printf("✅ 快照已生成：%s（%.1f KB）\n", snap.Name, float64(snap.Size)/1024)
	fmt.Println("位置:", config.GetDBFolderPath()+"/backups/"+snap.Name)
}
