package handler

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log"
	"scheduler/database"
	"scheduler/models"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func RencanaKerja(rd int) {
	dbGo := database.DB
	dbKbm := database.DBKBM

	var nationalHolidays []*models.THariLibur
	today := time.Now().Add(time.Hour * (7))
	tomorrow := today.AddDate(0, 0, rd)
	dateTomorrow := tomorrow.Format("2006-01-02")

	var excludeNiks []string
	if err := dbKbm.Table("t_cluster_pengajar x").
		Select("x.c_nik_pengajar").
		Where("x.c_status = 'Aktif'").
		Where("x.c_status_pengajar in ('PF', 'PK')").
		Find(&excludeNiks).Error; err != nil {
		log.Println(err)
		return
	}

	if err := dbGo.Model(models.THariLibur{}).
		Where("c_tanggal_awal >= ?", dateTomorrow).
		Where("c_tanggal_akhir <= ?", dateTomorrow).
		Find(&nationalHolidays).Error; err != nil {
		log.Println(err)
		return
	}

	if today.Weekday() == time.Sunday || len(nationalHolidays) > 0 {
		log.Println("Sekarang adalah hari libur")
		return
	}

	var niks []struct {
		Nik           string `json:"nik"`
		IdKewilayahan int    `json:"id_kewilayahan"`
	}

	if err := dbGo.Debug().
		Table("t_cluster_karyawan x").
		Select("x.c_nik as nik, z.c_id_kewilayahan as id_kewilayahan").
		Joins("LEFT JOIN t_jadwal_karyawan y ON y.c_nik = x.c_nik AND y.c_tanggal = ?", dateTomorrow).
		Joins("LEFT JOIN t_gokomar z ON z.c_id_komar = x.c_id_komar_cakupan").
		Where("x.c_nik not in ?", excludeNiks).
		Where("y.c_nik IS NULL").
		Scan(&niks).Error; err != nil {
		log.Println(err)
		return
	}

	if len(niks) == 0 {
		log.Println("Tidak ada karyawan yang perlu ditambahkan jadwal kerja")
		return
	}

	txGO := dbGo.Begin()
	defer func() {
		if r := recover(); r != nil {
			txGO.Rollback()
		}
	}()

	for _, kar := range niks {
		if kar.Nik == "" {
			continue
		}

		cJamAwal, cJamAkhir, cDurasiIstirahat := "00:00:00", "00:00:00", "00:00:00"
		cDurasiIstirahat = "01:00:00"
		if tomorrow.Weekday() == time.Friday {
			cDurasiIstirahat = "02:00:00"
		}

		if kar.IdKewilayahan == 1 {
			cJamAwal = "09:00:00"
			cJamAkhir = "17:00:00"
			if tomorrow.Weekday() == time.Saturday {
				cJamAkhir = "16:00:00"
			}
		} else {
			cJamAwal = "10:00:00"
			cJamAkhir = "18:00:00"
			if tomorrow.Weekday() == time.Saturday {
				// cJamAkhir = "18:00:00"
				cDurasiIstirahat = "02:00:00"
			}
		}

		updatedAt := time.Now().Format("2006-01-02 15:04:05.000")

		if err := txGO.Debug().Create(&models.TJadwalKaryawan{
			CNik:             kar.Nik,
			CTanggal:         &dateTomorrow,
			CJamAwal:         &cJamAwal,
			CJamAkhir:        &cJamAkhir,
			CDurasiIstirahat: &cDurasiIstirahat,
			CUpdater:         "scheduler",
			CLastUpdate:      &updatedAt,
		}).Error; err != nil {
			txGO.Rollback()
			log.Println(err)
			return
		}
	}

	if err := txGO.Commit().Error; err != nil {
		txGO.Rollback()
		log.Println(err)
		return
	}

	log.Println("Jadwal kerja ditambahkan untuk", len(niks), "karyawan")
}

type QueryParams struct {
	Page          int
	PerPage       int
	Keyword       string
	Status        string
	GedungID      int
	GedungIDs     []int
	KotaID        int
	SekretariatID int
	CabangID      int
	Nik           string
	Ids           []string
	IsAllData     bool
	Sort          string
}

func ProcessInfoKontak(jsonData string) models.InfoKontak {
	var info models.InfoKontak

	err := json.Unmarshal([]byte(jsonData), &info)
	if err != nil {
		log.Fatalf("Error parsing JSON: %v", err)
	}
	return info
}

func getExcelColumnName(index int) string {
	columnName := ""
	for index >= 0 {
		remainder := index % 26
		columnName = string(rune('A'+remainder)) + columnName
		index = (index / 26) - 1
	}
	return columnName
}

func FetchAndExportKaryawan(params QueryParams, db *gorm.DB) error {

	var karyawan []models.Karyawan

	query := db.Table("t_cluster_karyawan x").
		Select(`x.c_nik AS nik, x.c_nama_lengkap AS nama_lengkap, x.c_status AS status, 
			x.c_id_gedung AS gedung_id, x.c_id_kota AS kota_id, c.nama AS cabang, x.c_info_kontak as info_kontak, 
			x.c_tanggal_akhir_pkwt AS tanggal_akhir_pkwt,
			x.c_status_pengajar AS status_pengajar, x.sisa_cuti AS sisa_cuti,
			tbg.c_nama_bidang AS nama_bidang,
			tkw.c_nama_t_kewilayahan AS kewilayahan,
			tk.c_kota AS kota_kerja,
			se.nama AS sekretariat,
			bio.c_email AS email, bio.c_agama AS agama, bio.c_alamat AS alamat,
			j.c_jabatan_posisi AS jabatan, tgo.c_tanggal_mulai_kerja AS start_work, k.c_kelurahan AS kelurahan,
			kec.c_kecamatan AS kecamatan, tki.c_kota_indonesia AS kota, tpi.c_provinsi AS provinsi, 
			atas.c_nik AS atasan_nik, tck.c_nama_lengkap AS atasan_nama, z.c_nama_gedung AS nama_gedung
			`).
		Joins("LEFT JOIN t_gedung z ON x.c_id_gedung = z.c_id_gedung").
		Joins("LEFT JOIN cabang c ON z.c_id_cabang = c.id").
		Joins("LEFT JOIN t_jabatan_posisi j ON x.c_id_jabatan = j.c_id_jabatan_posisi").
		Joins("LEFT JOIN t_biodata_karyawan bio ON x.c_id_biodata = bio.c_id_biodata").
		Joins("LEFT JOIN t_karyawan_go tgo ON tgo.c_id_biodata = bio.c_id_biodata").
		Joins("LEFT JOIN t_kelurahan_indonesia k ON bio.c_id_kelurahan = k.c_id_kelurahan").
		Joins("LEFT JOIN t_kecamatan_indonesia kec ON k.c_id_kecamatan = kec.c_id_kecamatan").
		Joins("LEFT JOIN t_kota_indonesia tki ON tki.c_id_kota_indonesia = kec.c_id_kota_indonesia").
		Joins("LEFT JOIN t_kota tk ON x.c_id_kota = tk.c_id_kota").
		Joins("LEFT JOIN sekretariat se ON z.c_id_sekretariat = se.id").
		Joins("LEFT JOIN t_gokomar tg ON tg.c_id_komar = x.c_id_komar_cakupan").
		Joins("LEFT JOIN t_kewilayahan tkw ON tg.c_id_kewilayahan = tkw.c_id").
		Joins("LEFT JOIN t_provinsi_indonesia tpi ON tki.c_id_provinsi = tpi.c_id_provinsi").
		Joins("LEFT JOIN t_jabatan_pemangku p ON x.c_nik = p.c_nik and p.c_status = 'Aktif'").
		Joins("LEFT JOIN t_jabatan_pemangku atas ON atas.c_id_pejabat = p.c_id_pejabat_atasan").
		Joins("LEFT JOIN t_cluster_karyawan tck ON atas.c_nik = tck.c_nik").
		Joins("LEFT JOIN t_bidang_go tbg ON x.c_id_bidang = tbg.c_id_bidang")

	if params.Keyword != "" {
		query = query.Where("x.c_nama_lengkap ILIKE ?", "%"+params.Keyword+"%")
	}
	if params.Status != "" {
		query = query.Where("x.c_status = ?", params.Status)
	}
	if params.GedungID != 0 {
		query = query.Where("x.c_id_gedung = ?", params.GedungID)
	}
	if len(params.GedungIDs) > 0 {
		query = query.Where("x.c_id_gedung IN ?", params.GedungIDs)
	}
	if params.KotaID != 0 {
		query = query.Where("x.c_id_kota = ?", params.KotaID)
	}
	if params.Nik != "" {
		query = query.Where("x.c_nik = ?", params.Nik)
	}
	if len(params.Ids) > 0 {
		query = query.Where("x.c_nik IN ?", params.Ids)
	}

	sortColumn := "x.c_nama_lengkap"
	if params.Sort == "desc" {
		query = query.Order(sortColumn + " desc")
	} else {
		query = query.Order(sortColumn + " asc")
	}
	if !params.IsAllData {
		query = query.Offset(0).Limit(10)
	}

	if err := query.Scan(&karyawan).Error; err != nil {
		return err
	}

	file := excelize.NewFile()
	sheet := "Karyawan Data"
	index, err := file.NewSheet(sheet)
	if err != nil {
		fmt.Println(err)
	}

	headers := []string{"No", "NIK", "Nama Lengkap", "Status", "Gedung ID", "Gedung", "Kota ID", "Cabang", "Jabatan",
		"Tanggal Mulai Kerja", "Kelurahan", "Kecamatan", "Kota Lahir", "Provinsi", "Atasan NIK", "Atasan Nama", "HP",
		"Tanggal Akhir PKWT", "Status Pengajar", "Sisa Cuti", "Nama Bidang", "Email", "Agama", "Alamat",
		"Kewilayahan", "Kota Penempatan", "Sekretariat"}

	for i, header := range headers {
		cell := getExcelColumnName(i) + "1"
		file.SetCellValue(sheet, cell, header)
	}

	for i, kar := range karyawan {

		contactInfo := ProcessInfoKontak(string(kar.InfoKontak))
		row := i + 2

		file.SetCellValue(sheet, "A"+strconv.Itoa(row), i+1)
		file.SetCellValue(sheet, "B"+strconv.Itoa(row), kar.Nik)
		file.SetCellValue(sheet, "C"+strconv.Itoa(row), kar.NamaLengkap)
		file.SetCellValue(sheet, "D"+strconv.Itoa(row), kar.Status)
		file.SetCellValue(sheet, "E"+strconv.Itoa(row), kar.GedungID)
		file.SetCellValue(sheet, "F"+strconv.Itoa(row), kar.NamaGedung)
		file.SetCellValue(sheet, "G"+strconv.Itoa(row), kar.KotaID)
		file.SetCellValue(sheet, "H"+strconv.Itoa(row), kar.Cabang)
		file.SetCellValue(sheet, "I"+strconv.Itoa(row), kar.Jabatan)
		formattedDate := kar.StartWork.Format("2006-01-02")
		file.SetCellValue(sheet, "J"+strconv.Itoa(row), formattedDate)
		file.SetCellValue(sheet, "K"+strconv.Itoa(row), kar.Kelurahan)
		file.SetCellValue(sheet, "L"+strconv.Itoa(row), kar.Kecamatan)
		file.SetCellValue(sheet, "M"+strconv.Itoa(row), kar.Kota)
		file.SetCellValue(sheet, "N"+strconv.Itoa(row), kar.Provinsi)
		file.SetCellValue(sheet, "O"+strconv.Itoa(row), kar.AtasanNik)
		file.SetCellValue(sheet, "P"+strconv.Itoa(row), kar.AtasanNama)
		file.SetCellValue(sheet, "Q"+strconv.Itoa(row), contactInfo.HP)
		if kar.TanggalAkhirPKWT != nil {
			formattedAkhirDate := kar.TanggalAkhirPKWT.Format("2006-01-02")
			file.SetCellValue(sheet, "R"+strconv.Itoa(row), formattedAkhirDate)
		} else {
			file.SetCellValue(sheet, "R"+strconv.Itoa(row), "")
		}
		file.SetCellValue(sheet, "S"+strconv.Itoa(row), kar.StatusPengajar)
		file.SetCellValue(sheet, "T"+strconv.Itoa(row), kar.SisaCuti)
		file.SetCellValue(sheet, "U"+strconv.Itoa(row), kar.NamaBidang)
		file.SetCellValue(sheet, "V"+strconv.Itoa(row), kar.Email)
		file.SetCellValue(sheet, "W"+strconv.Itoa(row), kar.Agama)
		file.SetCellValue(sheet, "X"+strconv.Itoa(row), kar.Alamat)
		file.SetCellValue(sheet, "Y"+strconv.Itoa(row), kar.Kewilayahan)
		file.SetCellValue(sheet, "Z"+strconv.Itoa(row), kar.KotaKerja)
		file.SetCellValue(sheet, "AA"+strconv.Itoa(row), kar.Sekretariat)
	}

	file.SetActiveSheet(index)

	if err := file.SaveAs("KaryawanData.xlsx"); err != nil {
		return err
	}

	log.Println("Excel file generated successfully")
	return nil
}

type QueryParamsTobk struct {
	list_kode_tob string
	tahun_ajaran  string
	tanggal_awal  string
	tanggal_akhir string
}

func NewQueryParamsTobk() QueryParamsTobk {
	return QueryParamsTobk{
		list_kode_tob: "105455",
		tahun_ajaran:  "2024/2025",
		tanggal_awal:  "2024-11-16",
		tanggal_akhir: "2024-11-18",
	}
}

type InfoNilai struct {
	FullCredit int `json:"fullcredit"`
	HalfCredit int `json:"halfcredit"`
	ZeroCredit int `json:"zerocredit"`
}

type DetilJawaban struct {
	NoRegister        string    `json:"no_register"`
	NamaJenisProduk   string    `json:"nama_jenis_produk"`
	KodeTob           int       `json:"kode_tob"`
	KodePaket         string    `json:"kode_paket"`
	TahunAjaran       string    `json:"tahun_ajaran"`
	IDKelompokUjian   int       `json:"id_kelompok_ujian"`
	NamaKelompokUjian string    `json:"nama_kelompok_ujian"`
	Nilai             int       `json:"nilai"`
	InfoNilai         InfoNilai `json:"info_nilai"`
}

type Result struct {
	CreatedAt    string         `json:"created_at"`
	DetilHasil   []interface{}  `json:"detil_hasil"`
	DetilJawaban []DetilJawaban `json:"detil_jawaban"`
}

type OpsiStruct struct {
	Prop  string `json:"prop"`
	Opsi  any    `json:"opsi"`
	Nilai struct {
		Fullcredit int `json:"fullcredit"`
		Halfcredit int `json:"halfcredit"`
		Zerocredit int `json:"zerocredit"`
	} `json:"nilai"`
}

type QueryResult struct {
	KodePaket         string     `gorm:"column:c_kode_paket" json:"c_kode_paket"`
	IDSoal            int        `gorm:"column:c_id_soal"`
	NomorSoal         int        `gorm:"column:c_nomor_soal"`
	NamaKelompokUjian string     `gorm:"column:c_nama_kelompok_ujian"`
	TipeSoal          string     `gorm:"column:c_tipe_soal"`
	Opsi              OpsiStruct `gorm:"column:c_opsi"`
}

func (a *OpsiStruct) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, &a)
}

func (a OpsiStruct) Value() (driver.Value, error) {
	return json.Marshal(a)
}

func FetchDetilJawabanD(db *gorm.DB) error {
	params := NewQueryParamsTobk()

	dbTobk := database.DBTOBK

	query := dbTobk.Table("hasil_jawaban hj")

	if params.tanggal_awal != "" && params.tanggal_akhir != "" {
		query = query.Where("hj.updated_at BETWEEN ? AND ?", params.tanggal_awal, params.tanggal_akhir)
	}

	if params.tahun_ajaran != "" {
		query = query.Where("hj.tahun_ajaran = ?", params.tahun_ajaran)
	}

	if len(params.list_kode_tob) > 0 {
		listKodeTob := strings.Split(params.list_kode_tob, ",")
		query = query.Where("hj.kode_tob IN ?", listKodeTob)
	}

	var results []map[string]interface{}
	err := query.Find(&results).Error
	if err != nil {
		log.Fatalf("Failed to fetch records: %v", err)
		return err
	}

	listKodeTob := strings.Split(params.list_kode_tob, ",")

	kodeTob := listKodeTob

	var queryResults []QueryResult
	if err := dbTobk.Debug().
		Table("t_isi_tob tit").
		Select("tit.c_kode_paket as c_kode_paket, ts.c_id_soal, tibs.c_nomor_soal, tku.c_nama_kelompok_ujian, ts.c_tipe_soal, ts.c_opsi").
		Joins("JOIN t_paket_dan_bundel tpdb on tit.c_kode_paket = tpdb.c_kode_paket").
		Joins("JOIN t_bundel_soal tbs on tpdb.c_id_bundel = tbs.c_id_bundel").
		Joins("JOIN t_isi_bundel_soal tibs on tbs.c_id_bundel = tibs.c_id_bundel").
		Joins("JOIN t_soal ts on tibs.c_id_soal = ts.c_id_soal").
		Joins("JOIN t_kelompok_ujian tku on tbs.c_id_kelompok_ujian = tku.c_id_kelompok_ujian").
		Where("tit.c_kode_tob IN (?)", kodeTob).
		Order("tit.c_nomor_urut asc, tpdb.c_urutan asc, tibs.c_nomor_soal asc").
		Scan(&queryResults).Error; err != nil {
		log.Println(err)
		return err
	}

	hhah, _ := json.Marshal(queryResults)
	fmt.Println(string(hhah))

	// log.Printf("Query Results: %+v", queryResults)

	// for _, result := range queryResults {
	// 	fmt.Printf("Ops: %s\n", string(result.Opsi))
	// }

	// err = dbTobk.Raw(rawSQL, kodeTob).Scan(&queryResults).Error
	// if err != nil {
	// 	log.Fatalf("Failed to execute raw SQL query: %v", err)
	// 	return err
	// }
	// log.Printf("Executing Query: %s, Params: %+v", rawSQL, listKodeTob)

	// log.Printf("Query Results: %+v", queryResults)

	return nil
}
