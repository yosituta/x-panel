package model

// SubGroup 【开源增强】订阅系统：把多个外部节点链接聚合成一个订阅
//
// 与「每个客户端一个 subId」不同，订阅组是独立的：
// 你可以在任意服务器的面板里建组，把各台机器的分享链接粘进来，
// 组内节点可自由增删，最终得到一条订阅链接，客户端扫一次拿到全部节点。
type SubGroup struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string `json:"name" gorm:"default:''"`
	Token     string `json:"token" gorm:"uniqueIndex;default:''"`
	Nodes     string `json:"nodes" gorm:"type:text"` // 每行一个分享链接
	Remark    string `json:"remark" gorm:"default:''"`
	CreatedAt int64  `json:"createdAt" gorm:"default:0"`
	UpdatedAt int64  `json:"updatedAt" gorm:"default:0"`
}

func (SubGroup) TableName() string {
	return "sub_groups"
}
