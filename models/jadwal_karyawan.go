package models

type TJadwalKaryawan struct {
	CNik             string  `gorm:"primaryKey;size:255"`
	CTanggal         *string `gorm:"primaryKey;type:date"`
	CJamAwal         *string `gorm:"type:time"`
	CJamAkhir        *string `gorm:"type:time"`
	CDurasiIstirahat *string `gorm:"type:time"`
	CUpdater         string  `gorm:"size:255"`
	CVerifikasi      *int    `gorm:"default:null"`
	CCreatedAt       *string `gorm:"type:timestamptz"`
	CLastUpdate      *string `gorm:"type:timestamptz"`
}

func (TJadwalKaryawan) TableName() string {
	return "t_jadwal_karyawan"
}
