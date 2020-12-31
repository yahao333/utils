package mgo

import (
	"os"
	"sync"
	"time"

	log "github.com/yahao333/utils/logd"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type M bson.M

var (
	globalMS *mgo.Session
	mu       sync.RWMutex

	ErrNotFound = mgo.ErrNotFound
)

const (
	DEFAULT_MGO_TIMEOUT = 15
	DEFAULT_MGO_ADDR    = "127.0.0.1:27017"
)

func init() {
	mgoAddr := DEFAULT_MGO_ADDR
	if addr := os.Getenv("EIBLOG_MGO_ADDR"); addr != "" {
		mgoAddr = addr
	}
	sess, err := mgo.Dial(mgoAddr)
	if err != nil {
		log.Error(err)
	}

	cred := mgo.Credential{
		Username: "admin",
		Password: "admin",
	}
	sess.Login(&cred)

	sess.SetMode(mgo.Strong, true)
	sess.SetSocketTimeout(DEFAULT_MGO_TIMEOUT * time.Second)
	sess.SetCursorTimeout(0)
	globalMS = sess
}

func Connect(dataBase, collection string) (*mgo.Session, *mgo.Collection) {
	ms := globalMS.Copy()
	c := ms.DB(dataBase).C(collection)
	return ms, c
}

func Index(db, collection string, keys []string) error {
	ms, c := Connect(db, collection)
	defer ms.Close()

	index := mgo.Index{
		Key:        keys,
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}

	return c.EnsureIndex(index)
}

func KeyIsExist(db, collection, key, value string) bool {
	ms, c := Connect(db, collection)
	defer ms.Close()

	mu.Lock()
	defer mu.Unlock()
	count, err := c.Find(bson.M{key: value}).Count()
	if err == mgo.ErrNotFound || count > 0 {
		return true
	}
	if err != nil { // 查找出错, 为了以防万一还是返回存在
		log.Error(err)
		return true
	}
	return false
}

func IsEmpty(db, collection string) bool {
	ms, c := Connect(db, collection)
	defer ms.Close()
	count, err := c.Count()
	if err != nil {
		log.Error(err)
	}
	return count == 0
}

func Insert(db, collection string, docs interface{}) error {
	ms, c := Connect(db, collection)
	defer ms.Close()

	return c.Insert(docs)
}

func FindOne(db, collection string, selector, result interface{}) error {
	ms, c := Connect(db, collection)
	defer ms.Close()

	return c.Find(selector).One(result)
}

func FindAll(db, collection string, selector, result interface{}) error {
	ms, c := Connect(db, collection)
	defer ms.Close()

	return c.Find(selector).All(result)
}

func FindIter(db, collection string, selector interface{}) (*mgo.Iter, *mgo.Session) {
	ms, c := Connect(db, collection)
	return c.Find(selector).Iter(), ms
}

func Count(db, collection string, query interface{}) (int, error) {
	ms, c := Connect(db, collection)
	defer ms.Close()

	return c.Find(query).Count()
}

func Update(db, collection string, selector, update interface{}) error {
	ms, c := Connect(db, collection)
	defer ms.Close()

	err := c.Update(selector, update)
	return err
}

func Upsert(db, collection string, selector, update interface{}) error {
	ms, c := Connect(db, collection)
	defer ms.Close()

	_, err := c.Upsert(selector, update)
	return err
}

func Remove(db, collection string, selector interface{}) error {
	ms, c := Connect(db, collection)
	defer ms.Close()

	return c.Remove(selector)
}

type Counter struct {
	Name    string
	NextVal int64
}

func NextVal(db, countername string) int32 {
	ms, c := Connect(db, "COUNTERS")
	defer ms.Close()

	change := mgo.Change{
		Update:    bson.M{"$inc": bson.M{"nextval": 1}},
		Upsert:    true,
		ReturnNew: true,
	}

	next := &Counter{}
	info, err := c.Find(bson.M{"name": countername}).Apply(change, &next)
	if err != nil {
		log.Error(info, err)
		return -1
	}
	// round the nextval to 2^31
	return int32(next.NextVal % 2147483648)
}

func DeepCopy(val interface{}, newVal interface{}) {
	data, err := bson.Marshal(val)
	if err != nil {
		log.Error("bson.Marsal: ", err)
		return
	}

	if err := bson.Unmarshal(data, newVal); err != nil {
		log.Error("bson.Unmarshal: ", err)
		return
	}
}
