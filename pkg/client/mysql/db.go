package mysql

import (
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"joynova.com/library/supernova/pkg/jlog"
	"xorm.io/xorm"
	"xorm.io/xorm/contexts"
)

type Engine = xorm.Engine

type DB struct {
	engine *xorm.EngineGroup
	logger Logger
}

func (db *DB) SetLogger(l Logger) {
	db.logger = l
	db.engine.SetLogger(l)
}

// WriteEngine 获取用于写的库，读请求一定用读引擎!!!
func (db *DB) WriteEngine() *xorm.Engine {
	return db.engine.Master()
}

func (db *DB) Transaction(f func(tx *xorm.Session) (interface{}, error)) (interface{}, error) {
	return db.WriteEngine().Transaction(f)
}

// ReadEngine 获取用于读的库，写请求一定用写引擎!!!
func (db *DB) ReadEngine() *xorm.Engine {
	return db.engine.Slave()
}

func (db *DB) ReadGet(f func(engine *Engine) (bool, error)) (bool, error) {
	if len(db.engine.Slaves()) <= 0 {
		writeEngine := db.WriteEngine()
		return f(writeEngine)
	}
	readEngine := db.ReadEngine()
	has, err := f(readEngine)
	if err != nil {
		writeEngine := db.WriteEngine()
		return f(writeEngine)
	}

	return has, nil
}

func (db *DB) ReadFind(f func(engine *Engine) error) error {
	if len(db.engine.Slaves()) <= 0 {
		writeEngine := db.WriteEngine()
		return f(writeEngine)
	}
	readEngine := db.ReadEngine()
	err := f(readEngine)
	if err != nil {
		writeEngine := db.WriteEngine()
		return f(writeEngine)
	}

	return nil
}

func (db *DB) WriteExec(wo *LogWriteOp, f func(engine *Engine) (aff int, err error)) error {
	aff, err := f(db.WriteEngine())
	if err != nil {
		logger.ErrorWrite(wo, err, aff)
		return err
	}

	if wo != nil && wo.AffectedRows >= 0 && aff != wo.AffectedRows {
		logger.ErrorWrite(wo, err, aff)
		return fmt.Errorf("affected not equal:%v/%v", aff, wo.AffectedRows)
	}

	return nil
}

type Config struct {
	MasterDsn   string
	SlavesDsn   []string
	MaxOpenConn int
	MaxIdleConn int
}

func ParseParams2Dsn(addr, user, pwd, db string) string {
	return fmt.Sprintf("%v:%v@tcp(%v)/%v?charset=utf8&parseTime=True&loc=Local",
		user, pwd, addr, db)
}

func NewDB(conf *Config) (*DB, error) {
	engine, err := xorm.NewEngineGroup("mysql", append([]string{conf.MasterDsn}, conf.SlavesDsn...))
	if err != nil {
		return nil, err
	}

	if conf.MaxOpenConn <= 0 {
		conf.MaxOpenConn = 50
	}

	if conf.MaxIdleConn <= 0 {
		conf.MaxIdleConn = 10
	}

	if conf.MaxIdleConn > conf.MaxOpenConn {
		conf.MaxOpenConn = 50
		conf.MaxIdleConn = 10
	}

	jlog.Noticef("db start, max open conn:%v, max idle con:%v\n", conf.MaxOpenConn, conf.MaxIdleConn)

	engine.SetMaxOpenConns(conf.MaxOpenConn)
	engine.SetMaxIdleConns(conf.MaxIdleConn)
	engine.SetConnMaxLifetime(time.Minute * 14)

	go func() {
		ticker := time.NewTicker(time.Second * 60)
		for range ticker.C {
			engine.Ping()
		}
	}()

	db := &DB{
		engine: engine,
		logger: nil,
	}
	return db, nil
}

func (d *DB) AddHook(hook contexts.Hook) *DB {
	d.engine.AddHook(hook)
	return d
}
