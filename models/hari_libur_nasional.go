package models

import "time"

type THariLibur struct {
	IDHariLibur    int       `gorm:"column:c_id_hari_libur;primaryKey"`
	JenisHariLibur string    `gorm:"column:c_jenis_hari_libur"`
	NamaHariLibur  string    `gorm:"column:c_nama_hari_libur"`
	TanggalAwal    time.Time `gorm:"column:c_tanggal_awal"`
	TanggalAkhir   time.Time `gorm:"column:c_tanggal_akhir"`
	Kota           string    `gorm:"column:c_kota"`
	Siapa          string    `gorm:"column:c_siapa"`
	Updater        string    `gorm:"column:c_updater"`
	CreatedAt      time.Time `gorm:"column:c_created_at"`
	LastUpdate     time.Time `gorm:"column:c_last_update"`
}

// TableName sets the insert table name for this struct type
func (THariLibur) TableName() string {
	return "t_hari_libur"
}
