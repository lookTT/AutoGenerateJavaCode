package main

import (
	"AutoGenerateJavaCode/Interface"
	"AutoGenerateJavaCode/Model"
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
)

var db *sql.DB
var config Model.SConfig
var structTemplate Model.SStructTemplate
var mapperTemplate Model.SMapperTemplate
var mybatisTemplate Model.SMybatisTemplate
var ttTemplate *template.Template



func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

// 如果不存在的文件夹则创建
func checkDirectoryAndCreate(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// file or directory exists
		checkErr(os.MkdirAll(path, os.ModePerm))
	}
}

//初始化数据库连接
func initDB(config *Model.SConfig) error {
	urlPath := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8", config.UserName, config.PassWd, config.HOST, config.DBName)
	var err error
	db, err = sql.Open("mysql", urlPath)

	checkErr(err)
	db.SetMaxOpenConns(6000)
	db.SetMaxIdleConns(500)

	return nil
}

//小驼峰
func HandlingStringsLittle(in string) string {
	tmp := HandlingStringsBig(in)
	return fmt.Sprintf("%s%s", strings.ToLower(tmp[0:1]), tmp[1:])
}

//大驼峰
func HandlingStringsBig(in string) string {
	if in == "" {
		return in
	}
	var out bytes.Buffer
	sss := strings.Split(in, "_")
	var tmp string
	for _, s := range sss {
		tmp = s[0:1]
		out.WriteString(strings.ToUpper(tmp))
		out.WriteString(s[1:])
	}
	return out.String()
}

//处理结构体
func StructHandler(curTableName string, sFieldInfos []Model.SFieldInfo, size int) {
	var sFieldInfo Model.SFieldInfo
	var strLine, DATA string
	for i := 0; i < size; i++ {
		sFieldInfo = sFieldInfos[i]
		if _, ok := config.TypeTranslate[sFieldInfo.FieldType]; !ok {
			continue
		}
		strLine = config.TypeTranslate[sFieldInfo.FieldType]

		strLine = fmt.Sprintf(strLine, sFieldInfo.FieldNameCamel, sFieldInfo.FieldComment)
		DATA += strLine
	}
	structTemplate.CLASSNAME = HandlingStringsBig(curTableName)
	structTemplate.CLASSNAME = fmt.Sprintf("%s%s%s", config.ClassNamePrefix, structTemplate.CLASSNAME, config.ClassNameSuffix)
	if len(DATA) > 2 {
		DATA = DATA[:len(DATA)-2]
	}
	structTemplate.DATA = DATA

	f, err := os.Create(fmt.Sprintf("./target/%s.java", HandlingStringsBig(structTemplate.CLASSNAME)))
	checkErr(err)
	defer f.Close()
	checkErr(ttTemplate.ExecuteTemplate(f, "template_struct", structTemplate))
}

//处理mapper
func MapperHandler(curTableName string) {
	TableNameCamel := HandlingStringsBig(curTableName)
	TableNameCamel = fmt.Sprintf("%s%s%s", config.ClassNamePrefix, TableNameCamel, config.ClassNameSuffix)
	mapperTemplate.PackageStruct = config.PackageStruct + "." + TableNameCamel
	mapperTemplate.InterfaceName = TableNameCamel + "Mapper"
	mapperTemplate.StructName = TableNameCamel

	f, err := os.Create(fmt.Sprintf("./target/%s.java", mapperTemplate.InterfaceName))
	checkErr(err)
	defer f.Close()
	checkErr(ttTemplate.ExecuteTemplate(f, "template_mapper", mapperTemplate))
}

//处理Mybatis
func MybatisHandler(curTableName string, sFieldInfos []Model.SFieldInfo, size int) {
	var sFieldInfo Model.SFieldInfo
	var CustomResultMapBuffer bytes.Buffer
	var TableColumnsBuffer bytes.Buffer
	var EntityPropertiesBuffer bytes.Buffer
	var BatchEntityPropertiesBuffer bytes.Buffer
	var UpdateContentBuffer bytes.Buffer
	var LimitContentBuffer bytes.Buffer
	strLimit := `            <!-- 
            <if test="id != null">
                AND id=#{id}
            </if>
            <if test="idList != null">
                AND id IN
                <foreach collection="idList" item="it" separator="," open="(" close=")">
                    #{it}
                </foreach>
            </if>
            -->
`
	LimitContentBuffer.WriteString(strLimit)
	for i := 0; i < size; i++ {
		sFieldInfo = sFieldInfos[i]
		CustomResultMapBuffer.WriteString(fmt.Sprintf("        <result property=\"%s\" column=\"%s\"/>\r\n", sFieldInfo.FieldNameCamel, sFieldInfo.FieldName))
		TableColumnsBuffer.WriteString(fmt.Sprintf("        %s,\r\n", sFieldInfo.FieldName))
		EntityPropertiesBuffer.WriteString(fmt.Sprintf("        #{%s},\r\n", sFieldInfo.FieldNameCamel))
		BatchEntityPropertiesBuffer.WriteString(fmt.Sprintf("        #{item.%s},\r\n", sFieldInfo.FieldNameCamel))

		if "char" == sFieldInfo.FieldType ||
			"varchar" == sFieldInfo.FieldType ||
			"tinytext" == sFieldInfo.FieldType ||
			"text" == sFieldInfo.FieldType ||
			"mediumtext" == sFieldInfo.FieldType ||
			"longtext" == sFieldInfo.FieldType {
			//字符串类型
			//UpdateContentBuffer.WriteString(fmt.Sprintf("            <if test=\"%s != null and %s != ''\">%s = #{%s}</if>\r\n", sFieldInfo.FieldNameCamel, sFieldInfo.FieldNameCamel, sFieldInfo.FieldName, sFieldInfo.FieldNameCamel))
			//LimitContentBuffer.WriteString(fmt.Sprintf("            <if test=\"%s != null and %s != ''\">AND %s = #{%s}</if>\r\n", sFieldInfo.FieldNameCamel, sFieldInfo.FieldNameCamel, sFieldInfo.FieldName, sFieldInfo.FieldNameCamel))
			UpdateContentBuffer.WriteString(fmt.Sprintf("            <if test=\"%s != null and %s != ''\">%s = #{%s},</if>\r\n", sFieldInfo.FieldNameCamel, sFieldInfo.FieldNameCamel, sFieldInfo.FieldName, sFieldInfo.FieldNameCamel))
			LimitContentBuffer.WriteString(fmt.Sprintf("            <if test=\"%s != null and %s != ''\">AND %s LIKE CONCAT(CONCAT('%%',#{%s},'%%'))</if>\r\n", sFieldInfo.FieldNameCamel, sFieldInfo.FieldNameCamel, sFieldInfo.FieldName, sFieldInfo.FieldNameCamel))
		} else {
			//非字符串
			UpdateContentBuffer.WriteString(fmt.Sprintf("            <if test=\"%s != null\">%s = #{%s},</if>\r\n", sFieldInfo.FieldNameCamel, sFieldInfo.FieldName, sFieldInfo.FieldNameCamel))
			LimitContentBuffer.WriteString(fmt.Sprintf("            <if test=\"%s != null\">AND %s = #{%s}</if>\r\n", sFieldInfo.FieldNameCamel, sFieldInfo.FieldName, sFieldInfo.FieldNameCamel))
		}

	}
	TableNameCamel := HandlingStringsBig(curTableName)
	TableNameCamel = fmt.Sprintf("%s%s%s", config.ClassNamePrefix, TableNameCamel, config.ClassNameSuffix)
	mybatisTemplate.TableName = curTableName
	mybatisTemplate.MapperPath = config.PackageMapper + "." + TableNameCamel + "Mapper"
	mybatisTemplate.StructPath = config.PackageStruct + "." + TableNameCamel
	mybatisTemplate.CustomResultMap = CustomResultMapBuffer.String()
	mybatisTemplate.CustomResultMap = mybatisTemplate.CustomResultMap[:len(mybatisTemplate.CustomResultMap)-2]
	//处理一下 START
	mybatisTemplate.TableColumns = TableColumnsBuffer.String()
	mybatisTemplate.TableColumns = mybatisTemplate.TableColumns[:len(mybatisTemplate.TableColumns)-3]

	mybatisTemplate.EntityProperties = EntityPropertiesBuffer.String()
	mybatisTemplate.EntityProperties = mybatisTemplate.EntityProperties[:len(mybatisTemplate.EntityProperties)-3]

	mybatisTemplate.BatchEntityProperties = BatchEntityPropertiesBuffer.String()
	mybatisTemplate.BatchEntityProperties = mybatisTemplate.BatchEntityProperties[:len(mybatisTemplate.BatchEntityProperties)-3]
	//处理一下 END

	mybatisTemplate.UpdateContent = UpdateContentBuffer.String()
	mybatisTemplate.UpdateContent = mybatisTemplate.UpdateContent[:len(mybatisTemplate.UpdateContent)-2]
	mybatisTemplate.LimitContent = LimitContentBuffer.String()
	mybatisTemplate.LimitContent = mybatisTemplate.LimitContent[:len(mybatisTemplate.LimitContent)-2]

	f, err := os.Create(fmt.Sprintf("./target/%s.xml", TableNameCamel+"Mapper"))
	checkErr(err)
	defer f.Close()
	checkErr(ttTemplate.ExecuteTemplate(f, "template_mybatis", mybatisTemplate))
}

//处理表
func TableProcessing(curDBName string, curTableName string) {
	if curDBName == "" || curTableName == "" {
		return
	}

	strSql := fmt.Sprintf("SELECT `COLUMN_NAME`, `DATA_TYPE`, `COLUMN_COMMENT` from information_schema.columns WHERE table_schema = '%s' AND table_name = '%s' ORDER BY ordinal_position", curDBName, curTableName)
	rows, err := db.Query(strSql)
	checkErr(err)
	defer rows.Close()

	var sFieldInfos []Model.SFieldInfo

	var fieldName sql.NullString
	var fieldType sql.NullString
	var fieldComment sql.NullString
	for rows.Next() {
		_, err = rows.Columns()
		checkErr(err)
		err = rows.Scan(
			&fieldName,
			&fieldType,
			&fieldComment,
		)

		if !fieldName.Valid || !fieldType.Valid {
			continue
		}
		fieldName.String = strings.ToLower(fieldName.String)
		fieldType.String = strings.ToLower(fieldType.String)

		var sFieldInfo Model.SFieldInfo
		sFieldInfo.FieldName = fieldName.String
		sFieldInfo.FieldNameCamel = HandlingStringsLittle(fieldName.String)
		sFieldInfo.FieldType = fieldType.String
		sFieldInfo.FieldComment = fieldComment.String
		sFieldInfos = append(sFieldInfos, sFieldInfo)
	}

	StructHandler(curTableName, sFieldInfos, len(sFieldInfos))
	MapperHandler(curTableName)
	MybatisHandler(curTableName, sFieldInfos, len(sFieldInfos))
	Interface.DoSomeWork(&config, curTableName, sFieldInfos, len(sFieldInfos))
}

//处理数据库
func DatabaseProcessing(curDBName string, pWG *sync.WaitGroup) {
	if nil != pWG {
		defer pWG.Done()
	}
	if curDBName == "" {
		return
	}
	strSql := fmt.Sprintf("SHOW TABLE STATUS FROM `%s`;", curDBName)
	rows, err := db.Query(strSql)
	checkErr(err)
	defer rows.Close()

	var curTableName sql.NullString
	var tmp sql.NullString
	for rows.Next() {
		_, err = rows.Columns()
		checkErr(err)
		err = rows.Scan(
			&curTableName,
			&tmp, &tmp, &tmp, &tmp, &tmp, &tmp, &tmp, &tmp, &tmp, &tmp, &tmp, &tmp, &tmp, &tmp, &tmp, &tmp, &tmp,
		)
		checkErr(err)
		TableProcessing(curDBName, curTableName.String)
	}
}

func initTemplate() {
	strTemplate, err := ioutil.ReadFile("./template_struct.txt")
	checkErr(err)
	ttTemplate, err = template.New("template_struct").Parse(string(strTemplate))
	checkErr(err)

	strTemplate, err = ioutil.ReadFile("./template_mapper.txt")
	checkErr(err)
	ttTemplate, err = ttTemplate.New("template_mapper").Parse(string(strTemplate))
	checkErr(err)

	strTemplate, err = ioutil.ReadFile("./template_mybatis.txt")
	checkErr(err)
	ttTemplate, err = ttTemplate.New("template_mybatis").Parse(string(strTemplate))
	checkErr(err)

	now := time.Now()
	strDate := fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	structTemplate.PACKAGE = config.PackageStruct
	structTemplate.DATE = strDate
	structTemplate.AUTHOR = config.AUTHOR

	mapperTemplate.PackageMapper = config.PackageMapper
	//mapperTemplate.PackageStruct = config.PackageStruct
	mapperTemplate.AUTHOR = config.AUTHOR
	mapperTemplate.DATE = strDate
}

func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./")
	checkErr(viper.ReadInConfig())
	checkErr(viper.Unmarshal(&config))
	if nil != config.ExcludeDBName && len(config.ExcludeDBName) > 0 {
		config.ExcludeDBNameMap = make(map[string]int)
		for i := 0; i < len(config.ExcludeDBName); i++ {
			config.ExcludeDBNameMap[config.ExcludeDBName[i]] = 1
		}
	}
	if nil != config.IncludeDBName && len(config.IncludeDBName) > 0 {
		config.IncludeDBNameMap = make(map[string]int)
		for i := 0; i < len(config.IncludeDBName); i++ {
			config.IncludeDBNameMap[config.IncludeDBName[i]] = 1
		}
	}

	//初始化模板
	initTemplate()

	//创建输出文件夹
	checkDirectoryAndCreate("./target")

	checkErr(initDB(&config))
	defer db.Close()

	//找到所有的库
	rows, err := db.Query("SHOW DATABASES;")
	checkErr(err)
	defer rows.Close()

	var wg sync.WaitGroup
	pWG := &wg
	pWG = nil

	var curDBName string
	for rows.Next() {
		_, err = rows.Columns()
		checkErr(err)
		err = rows.Scan(
			&curDBName,
		)
		checkErr(err)

		if config.ExcludeDBNameMap[curDBName] == 1 {
			continue
		}

		if len(config.IncludeDBNameMap) == 0 || config.IncludeDBNameMap[curDBName] == 1 {
			if nil != pWG {
				pWG.Add(1)
				go DatabaseProcessing(curDBName, pWG)
			} else {
				DatabaseProcessing(curDBName, nil)
			}
		}
	}
	if nil != pWG {
		pWG.Wait()
	}

	fmt.Println("Completed!!!")
}
