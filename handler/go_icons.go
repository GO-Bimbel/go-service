package handler

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log"
	"scheduler/database"
	"scheduler/models"
	"sort"
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
	NoRegister         string      `json:"no_register"`
	NamaJenisProduk    string      `json:"nama_jenis_produk"`
	KodeTob            int         `json:"kode_tob"`
	KodePaket          string      `json:"kode_paket"`
	TahunAjaran        string      `json:"tahun_ajaran"`
	KodeBab            string      `json:"kode_bab"`
	NomorSoalSiswa     int         `json:"nomor_soal_siswa"`
	NomorSoalDatabase  int         `json:"nomor_soal_database"`
	IDKelompokUjian    int         `json:"id_kelompok_ujian"`
	NamaKelompokUjian  string      `json:"nama_kelompok_ujian"`
	IDSoal             int         `json:"id_soal"`
	TipeSoal           string      `json:"tipe_soal"`
	TingkatKesulitan   int         `json:"tingkat_kesulitan"`
	KesempatanMenjawab interface{} `json:"kesempatan_menjawab"`
	Jawaban            interface{} `json:"jawaban"`
	KunciJawaban       interface{} `json:"kunci_jawaban"`
	TranslatorEpb      interface{} `json:"translator_epb"`
	JawabanEpb         interface{} `json:"jawaban_epb"`
	KunciJawabanEpb    interface{} `json:"kunci_jawaban_epb"`
	InfoNilai          struct {
		Fullcredit int `json:"fullcredit"`
		Halfcredit int `json:"halfcredit"`
		Zerocredit int `json:"zerocredit"`
	} `json:"info_nilai"`
	Nilai            float64   `json:"nilai"`
	IsRagu           bool      `json:"is_ragu"`
	SudahDikumpulkan bool      `json:"sudah_dikumpulkan"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type DetilHasil struct {
	IDKelompokUjian   int    `json:"id_kelompok_ujian"`
	NamaKelompokUjian string `json:"nama_kelompok_ujian"`
	Benar             int    `json:"benar"`
	Salah             int    `json:"salah"`
	Kosong            int    `json:"kosong"`
}

type Result struct {
	Id           int             `gorm:"column:id"`
	NoRegister   string          `gorm:"column:no_register"`
	KodeTob      int             `gorm:"column:kode_tob"`
	KodePaket    string          `gorm:"column:kode_paket"`
	TahunAjaran  string          `gorm:"column:tahun_ajaran"`
	DetilHasil   json.RawMessage `gorm:"column:detil_hasil"`
	DetilJawaban json.RawMessage `gorm:"column:detil_jawaban"`
}

type OpsiStruct struct {
	Prop    string        `json:"prop"`
	Opsi    any           `json:"opsi"`
	Soal    any           `json:"soal"`
	Keyword any           `json:"keyword"`
	Kolom   []interface{} `json:"kolom"`
	Kunci   any           `json:"kunci"`
	Nilai   struct {
		Fullcredit int `json:"fullcredit"`
		Halfcredit int `json:"halfcredit"`
		Zerocredit int `json:"zerocredit"`
	} `json:"nilai"`
}

type QueryResult struct {
	KodePaket         string     `gorm:"column:c_kode_paket"`
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

type Pbt struct {
	Opsi []struct {
		Text    string `json:"text"`
		Jawaban []struct {
			Urut    int  `json:"urut"`
			Jawaban bool `json:"jawaban"`
		} `json:"jawaban"`
	} `json:"opsi"`
	Prop  interface{} `json:"prop"`
	Kolom []struct {
		Urut  int    `json:"urut"`
		Judul string `json:"judul"`
	} `json:"kolom"`
	Nilai struct {
		Fullcredit int `json:"fullcredit"`
		Halfcredit int `json:"halfcredit"`
		Zerocredit int `json:"zerocredit"`
	} `json:"nilai"`
	Jumlahopsi  int `json:"jumlahopsi"`
	Jumlahkolom int `json:"jumlahkolom"`
}

type Jawaban struct {
	Urut    int  `json:"urut"`
	Jawaban bool `json:"jawaban"`
}

type Output struct {
	Noreg          string
	KodePaket      string
	IdSoal         int
	NoSoalDatabase int
	KelompokUjian  string
	JawabanSiswa   interface{}
	KunciJawaban   interface{}
}

type KunciJawaban struct {
	KunciJawaban interface{}
	IdSoal       int
}

func ProcessRawDetilJSON(jsonData string) []DetilHasil {
	var info []DetilHasil

	err := json.Unmarshal([]byte(jsonData), &info)
	if err != nil {
		log.Fatalf("Error parsing JSON: %v", err)
	}
	return info
}

func ProcessRawJawabanJSON(jsonData string) []DetilJawaban {
	var info []DetilJawaban

	err := json.Unmarshal([]byte(jsonData), &info)
	if err != nil {
		log.Fatalf("Error parsing JSON: %v", err)
	}

	for i := range info {
		switch v := info[i].KunciJawaban.(type) {
		case string:

			log.Printf("kunci_jawaban_epb is a string: %s", v)
		case []interface{}:

			log.Printf("kunci_jawaban_epb is an array: %+v", v)
		default:
			log.Printf("kunci_jawaban_epb has an unexpected type: %T", v)
		}
	}

	return info
}

func setKunciJawaban(tipeSoal string, opsi OpsiStruct) interface{} {
	switch tipeSoal {
	case "PGB":
		var kunciJawaban string
		for k, v := range opsi.Opsi.(map[string]interface{}) {
			if v.(map[string]interface{})["bobot"].(float64) == 100 {
				kunciJawaban = k
			}
		}
		return kunciJawaban

	case "PBT":
		var jawabanValue []int

		for _, v := range opsi.Opsi.([]interface{}) {

			marshalV, _ := json.Marshal(v)

			var data map[string]interface{}
			err := json.Unmarshal(marshalV, &data)
			if err != nil {
				log.Fatalf("Error unmarshalling JSON: %v", err)
			}

			dataJawaban := data["jawaban"].([]interface{})

			var urut []Jawaban

			for _, obj := range dataJawaban {

				marshalObj, err := json.Marshal(obj)
				if err != nil {
					log.Fatalf("Error marshalling obj: %v", err)
				}

				var jawaban Jawaban
				err = json.Unmarshal(marshalObj, &jawaban)
				if err != nil {
					log.Fatalf("Error unmarshalling obj to Jawaban: %v", err)
				}

				if jawaban.Jawaban {
					urut = append(urut, jawaban)
				}
			}

			if len(urut) > 0 {
				jawabanValue = append(jawabanValue, urut[0].Urut-1)
			} else {
				jawabanValue = append(jawabanValue, -1)
			}
		}

		return jawabanValue

	case "PBK", "PBCT":
		var kunciJawaban []map[string]interface{}

		for _, v := range opsi.Kunci.(map[string]interface{}) {
			if kunci, ok := v.(map[string]interface{}); ok {
				kunciJawaban = append(kunciJawaban, kunci)
			} else {
				log.Printf("Failed to assert value to map[string]interface{}: %v", v)
			}
		}
		return kunciJawaban

	case "PBM":
		var kunciJawabanPasangan []interface{}

		for _, v := range opsi.Opsi.([]map[string]interface{}) {
			if jodoh, ok := v["jodoh"]; ok {
				kunciJawabanPasangan = append(kunciJawabanPasangan, jodoh)
			} else {
				kunciJawabanPasangan = append(kunciJawabanPasangan, -1)
			}
		}

		return kunciJawabanPasangan

	case "ESSAY", "ESSAY NUMERIK":

		var kunciJawaban interface{}

		kunciJawaban = opsi.Keyword
		return kunciJawaban

	case "ESSAY MAJEMUK":

		var kunciJawabanMajemuk []interface{}

		for _, val := range opsi.Soal.([]map[string]interface{}) {

			if keywords, ok := val["keywords"]; ok {
				kunciJawabanMajemuk = append(kunciJawabanMajemuk, keywords)
			} else {
				kunciJawabanMajemuk = append(kunciJawabanMajemuk, -1)
			}
		}
		return kunciJawabanMajemuk

	// to do case "PBB" but missing data in db
	// case "PBB" :
	// 	var kunciJawabanAlasan interface{}

	// 	for _, val := range opsi.Opsi.(map[string]interface{}) {

	// 	}

	default:

		var kunci interface{}
		kunci = opsi.Kunci
		return kunci
	}
}
func setTranslatorEPB(opsi OpsiStruct) interface{} {

	var kolom []interface{}
	if opsi.Kolom != nil {
		kolom = opsi.Kolom
	} else {
		return nil
	}

	if len(kolom) == 0 {
		return kolom
	}

	kolomBaru := kolom[1:]

	var result []string
	for _, item := range kolomBaru {

		if kolomItem, ok := item.(map[string]interface{}); ok {
			judul, ok := kolomItem["judul"].(string)
			if !ok {
				continue
			}

			start := strings.Index(judul, "(")
			end := strings.Index(judul, ")")
			if end < 0 {
				end = start + 2
			}

			if start < 0 {
				result = append(result, strings.TrimSpace(judul)[:1])
			} else {
				result = append(result, strings.TrimSpace(judul[start+1:end]))
			}
		}
	}

	return result

}

func fillMissingAnswers(data []Output) []Output {

	questionMap := make(map[int]Output)

	for _, item := range data {
		questionMap[item.IdSoal] = item
	}

	noregSet := make(map[string]struct{})
	for _, item := range data {
		noregSet[item.Noreg] = struct{}{}
	}

	var noregList []string
	for noreg := range noregSet {
		noregList = append(noregList, noreg)
	}

	var result []Output

	for _, noreg := range noregList {
		for _, question := range questionMap {
			found := false
			for _, item := range data {
				if item.Noreg == noreg && item.IdSoal == question.IdSoal {
					result = append(result, item)
					found = true
					break
				}
			}
			if !found {

				result = append(result, Output{
					Noreg:          noreg,
					KodePaket:      question.KodePaket,
					IdSoal:         question.IdSoal,
					NoSoalDatabase: question.NoSoalDatabase,
					KelompokUjian:  question.KelompokUjian,
					JawabanSiswa:   nil,
				})
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Noreg == result[j].Noreg {
			return result[i].NoSoalDatabase < result[j].NoSoalDatabase
		}
		return result[i].Noreg < result[j].Noreg
	})

	return result
}

func setTranslateJawabanEPB(jawaban []int, translator []string) []string {
	var jawabanEPB []string

	for _, item := range jawaban {
		if item < 0 {
			jawabanEPB = append(jawabanEPB, "")
		} else if item >= 0 && item < len(translator) {
			jawabanEPB = append(jawabanEPB, translator[item])
		} else {
			jawabanEPB = append(jawabanEPB, "")
		}
	}

	return jawabanEPB
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

	var results []Result
	err := query.Debug().Find(&results).Error
	if err != nil {
		log.Fatalf("Failed to fetch records: %v", err)
		return err
	}

	var output []Output
	var idSoalList []int

	for _, item := range results {

		detilJawaban := ProcessRawJawabanJSON(string(item.DetilJawaban))

		if len(detilJawaban) > 0 {
			for _, jawaban := range detilJawaban {
				idSoalList = append(idSoalList, jawaban.IDSoal)

				newOutput := Output{
					Noreg:          jawaban.NoRegister,
					KodePaket:      jawaban.KodePaket,
					IdSoal:         jawaban.IDSoal,
					NoSoalDatabase: jawaban.NomorSoalDatabase,
					KelompokUjian:  jawaban.NamaKelompokUjian,
					JawabanSiswa:   jawaban.Jawaban,
				}
				output = append(output, newOutput)

			}
		} else {
			log.Printf("detilJawaban is empty for item: %+v", item)
		}
	}

	result := fillMissingAnswers(output)

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
		Where("ts.c_id_soal IN (?)", idSoalList).
		Order("tit.c_nomor_urut asc, tpdb.c_urutan asc, tibs.c_nomor_soal asc").
		Scan(&queryResults).Error; err != nil {
		log.Println(err)
		return err
	}

	var kunciJawabSoal []KunciJawaban

	for _, item := range queryResults {

		var translator_jawaban_epb interface{}

		kunciJawaban := setKunciJawaban(item.TipeSoal, item.Opsi)

		if item.TipeSoal == "PBT" {

			translate_epb := setTranslatorEPB(item.Opsi)
			translate_jawaban_epb := setTranslateJawabanEPB(kunciJawaban.([]int), translate_epb.([]string))
			translator_jawaban_epb = translate_jawaban_epb

		}

		newOutput := KunciJawaban{
			KunciJawaban: func() interface{} {
				if translator_jawaban_epb != nil {
					return translator_jawaban_epb
				}
				return kunciJawaban
			}(),
			IdSoal: item.IDSoal,
		}
		kunciJawabSoal = append(kunciJawabSoal, newOutput)

	}

	for i := range result {
		for _, kunci := range kunciJawabSoal {
			if result[i].IdSoal == kunci.IdSoal {
				result[i].KunciJawaban = kunci.KunciJawaban
				break
			}
		}
	}

	file := excelize.NewFile()
	sheet := "Karyawan Data"
	index, err := file.NewSheet(sheet)
	if err != nil {
		fmt.Println(err)
	}

	headers := []string{"No", "Nomor Register", "Kode Paket", "ID Soal", "Nomor Soal Database", "Kelompok Ujian", "Jawaban Siswa EPB", "Kunci Jawaban EPB"}

	for i, header := range headers {
		cell := getExcelColumnName(i) + "1"
		file.SetCellValue(sheet, cell, header)
	}

	for i, kar := range result {

		row := i + 2
		file.SetCellValue(sheet, "A"+strconv.Itoa(row), i+1)
		file.SetCellValue(sheet, "B"+strconv.Itoa(row), kar.Noreg)
		file.SetCellValue(sheet, "C"+strconv.Itoa(row), kar.KodePaket)
		file.SetCellValue(sheet, "D"+strconv.Itoa(row), kar.IdSoal)
		file.SetCellValue(sheet, "E"+strconv.Itoa(row), kar.NoSoalDatabase)
		file.SetCellValue(sheet, "F"+strconv.Itoa(row), kar.KelompokUjian)
		file.SetCellValue(sheet, "G"+strconv.Itoa(row), kar.JawabanSiswa)
		file.SetCellValue(sheet, "H"+strconv.Itoa(row), kar.KunciJawaban)

	}

	file.SetActiveSheet(index)

	if err := file.SaveAs("Detil-Jawaban-D-TOBk.xlsx"); err != nil {
		return err
	}

	log.Println("Excel file generated successfully")

	return nil
}
