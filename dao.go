package main

import (
  "database/sql"
	_ "github.com/ziutek/mymysql/godrv"
	"log"
)

type SvnDao struct {
	db *sql.DB
}

type TUser struct {
	uid            uint
	uname          string
	lastModifyTime uint
}

type TRepos struct {
	rid            uint
	name           string
	lastModifyTime uint
}

type TPath struct {
	pid  uint
	rid  uint
	path string
}

func NewSvnDao(dbhost string) *SvnDao {
	dao := new(SvnDao)
	db, err := sql.Open("mymysql", dbhost)
	if err != nil {
		panic(err)
	}
	dao.db = db
	return dao
}

func (dao *SvnDao) getModifiedUser(lastModifyTime uint) map[uint]*TUser {
	//log.Printf("select uid, uname, last_modify_time from t_svn_user where last_modify_time > %d and status = 0", lastModifyTime)
	rows, err := dao.db.Query("select uid, uname, last_modify_time from t_svn_user where last_modify_time > ? and status = 0", lastModifyTime)
	if err != nil {
		log.Panic(err)
	}

	userMap := make(map[uint]*TUser)
	for rows.Next() && rows.Err() == nil {
		user := new(TUser)
		rows.Scan(&user.uid, &user.uname, &user.lastModifyTime)
		userMap[user.uid] = user
	}

	return userMap
}

func (dao *SvnDao) getUser(uid uint) *TUser {
	//log.Printf("select uid, uname, last_modify_time from t_svn_user where uid = %d and status = 0", uid)
	row := dao.db.QueryRow("select uid, uname, last_modify_time from t_svn_user where uid = ? and status = 0", uid)
	if row == nil {
		return nil
	}
	user := new(TUser)
	row.Scan(&user.uid, &user.uname, &user.lastModifyTime)
	return user
}

func (dao *SvnDao) getReposList() map[uint]string {
	//log.Printf("select rid, name from t_svn_repos where status = 0")
	rows, err := dao.db.Query("select rid, name from t_svn_repos where status = 0")
	if err != nil {
		log.Panic(err)
	}

	var rid uint
	var name string
	reposMap := make(map[uint]string)
	for rows.Next() && rows.Err() == nil {
		rows.Scan(&rid, &name)
		reposMap[rid] = name
	}
	return reposMap
}

func (dao *SvnDao) getModifiedReposList(lastModifyTime uint) map[uint]*TRepos {
	//log.Printf("select rid, name, last_modify_time from t_svn_repos where status = 0 and last_modify_time > %d", lastModifyTime)
	rows, err := dao.db.Query("select rid, name, last_modify_time from t_svn_repos where status = 0 and last_modify_time > ?", lastModifyTime)
	if err != nil {
		log.Panic(err)
	}

	reposMap := make(map[uint]*TRepos)
	for rows.Next() && rows.Err() == nil {
		repos := new(TRepos)
		rows.Scan(&repos.rid, &repos.name, &repos.lastModifyTime)
		reposMap[repos.rid] = repos
	}
	return reposMap
}

func (dao *SvnDao) getPathList(rid uint) map[uint]*TPath {
	//log.Printf("select pid, rid, path from t_svn_path where rid = %d", rid)
	rows, err := dao.db.Query("select pid, rid, path from t_svn_path where rid = ?", rid)
	if err != nil {
		log.Panic(err)
	}

	pathMap := make(map[uint]*TPath)
	for rows.Next() && rows.Err() == nil {
		path := new(TPath)
		rows.Scan(&path.pid, &path.rid, &path.path)
		pathMap[path.pid] = path
	}
	return pathMap
}

func (dao *SvnDao) getPermPathList(uid uint) map[uint]uint {
	//log.Printf("select pid, perm from t_svn_perm_path where status = 0 and uid = %d", uid)
	rows, err := dao.db.Query("select pid, perm from t_svn_perm_path where status = 0 and uid = ?", uid)
	if err != nil {
		log.Panic(err)
	}

	var pid, perm uint
	permPathMap := make(map[uint]uint)
	for rows.Next() && rows.Err() == nil {
		rows.Scan(&pid, &perm)
		permPathMap[pid] = perm
	}

	return permPathMap
}

/* vim: set ts=4 sw=4 sts=4 tw=100 noet: */
