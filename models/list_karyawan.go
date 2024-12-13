package models

import (
	"encoding/json"
	"time"
)

type InfoKontak struct {
	HP   string `json:"HP"`
	HP2  string `json:"HP2"`
	TLP  string `json:"TLP"`
	TLP2 string `json:"TLP2"`
}

type Karyawan struct {
	Nik              string          `gorm:"column:nik"`
	NamaLengkap      string          `gorm:"column:nama_lengkap"`
	Status           string          `gorm:"column:status"`
	GedungID         int             `gorm:"column:gedung_id"`
	KotaID           int             `gorm:"column:kota_id"`
	Cabang           string          `gorm:"column:cabang"`
	Jabatan          string          `gorm:"column:jabatan"`
	StartWork        time.Time       `gorm:"column:start_work"`
	Kelurahan        string          `gorm:"column:kelurahan"`
	Kecamatan        string          `gorm:"column:kecamatan"`
	Kota             string          `gorm:"column:kota"`
	Provinsi         string          `gorm:"column:provinsi"`
	AtasanNik        string          `gorm:"column:atasan_nik"`
	AtasanNama       string          `gorm:"column:atasan_nama"`
	NamaGedung       string          `gorm:"column:nama_gedung"`
	InfoKontak       json.RawMessage `gorm:"column:info_kontak"`
	TanggalAkhirPKWT *time.Time      `gorm:"column:tanggal_akhir_pkwt"`
	StatusPengajar   string          `gorm:"column:status_pengajar"`
	SisaCuti         int             `gorm:"column:sisa_cuti"`
	NamaBidang       string          `gorm:"column:nama_bidang"`
	Email            string          `gorm:"column:email"`
	Agama            string          `gorm:"column:agama"`
	Alamat           string          `gorm:"column:alamat"`
	Kewilayahan      string          `gorm:"column:kewilayahan"`
	KotaKerja        string          `gorm:"column:kota_kerja"`
	Sekretariat      string          `gorm:"column:sekretariat"`
}

func (Karyawan) TableName() string {
	return "t_cluster_karyawan"
}
