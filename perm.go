package main

import (
  "log"
	"net/url"
	"strings"
	"time"
)

const (
	PERM_READ  = 1
	PERM_WRITE = 1 << 1
)

type PathNode struct {
	name     string
	children map[string]*PathNode
	pid      map[uint]bool
}

func (root *PathNode) getPid(path string) map[uint]bool {
	pathList := strings.Split(path, "/")
	log.Printf("path:%s split into %d segments", path, len(pathList))
	node := root
	pid := make(map[uint]bool)
	for _, v := range pathList {
		//子结点不存在，说明查询路径是当前路径的子结点
		v, err := url.QueryUnescape(v)
		if err != nil {
			log.Printf("invalid url segment:%s", v)
			return nil
		}

		if node.children == nil {
			log.Printf("node:%s has no children, terminate on:%s", node.name, v)
			break
		}

		//子结点不存在，则查询路径不符合
		child, ok := node.children[v]
		if !ok {
			log.Printf("node:%s not found in parent:%s", v, node.name)
			break
		}

		if child.pid == nil || len(child.pid) == 0 {
			log.Printf("node:%s has no pid defined", child.name)
			continue
		}

		for k, v := range child.pid {
			pid[k] = v
			log.Printf("node:%s has pid:%d defined", child.name, k)
		}

		node = child
	}

	return pid
}

//将一个路径加到查询路径中
func (root *PathNode) add(path string, pid uint) {
	pathList := strings.Split(path, "/")
	node := root
	for _, v := range pathList {
		//排除诸如"a//b//c这种情形
		if v == "" {
			continue
		}

		v, err := url.QueryUnescape(v)
		if err != nil {
			log.Printf("invalid path:%s, ignored", path)
			continue
		}

		//如果当前模块还没有子结点，则创建子结点
		if node.children == nil {
			node.children = make(map[string]*PathNode)
		}

		//查看当前路径不存在时，则加到路径中
		child, ok := node.children[v]
		if !ok {
			child = &PathNode{
				name: v,
				pid:  make(map[uint]bool),
			}
			node.children[v] = child
			log.Printf("add child:%s for node:%s", v, node.name)
		}
		node = child
	}

	node.pid[pid] = true
}

type ReposRoot struct {
	name string
	rid  uint
	node *PathNode
}

type ReposParent struct {
	refreshInterval    time.Duration
	lastUserCheckTime  uint
	lastReposCheckTime uint
	reposList          map[uint]*ReposRoot
	prefix             string
	users              map[string]*User
	dao                *SvnDao
	ignoreList         []string
}

func NewReposRoot(rid uint, name string) *ReposRoot {
	return &ReposRoot{
		rid:  rid,
		name: name,
		node: &PathNode{
			name: name,
			pid:  make(map[uint]bool),
		},
	}
}

func (parent *ReposParent) shouldPass(path string) bool {
	if parent.ignoreList == nil {
		return false
	}
	for _, v := range parent.ignoreList {
		if path == v {
			return true
		}
	}
	return false
}

func (parent *ReposParent) CanAccess(username string, method string, path string) bool {

	if parent.shouldPass(path) {
		return true
	}
	if parent.users == nil {
		log.Printf("server initing")
		return false
	}

	user, ok := parent.users[username]
	if !ok {
		log.Printf("user:%s not found", username)
		return false
	}

	pathes := strings.Split(path, "!")
	path = pathes[0]
	pathes = strings.SplitN(path, "/", 3)
	if len(pathes) < 3 {
		log.Printf("unexpected path:%s, pass through", path)
		return true
	}
	repos := pathes[1]
	path = pathes[2]
	rid := parent.getRid(repos)
	if rid == 0 {
		log.Printf("repos:%s not found", repos)
		return false
	}

	if path == "" {
		return true
	}

	pid := parent.reposList[rid].node.getPid(path)
	if pid == nil {
		log.Printf("path:%s not found in repos:%s", path, repos)
		return false
	}

	perm := user.getPerm(pid)
	mask, ok := MethodMap[method]
	log.Printf("user:%s, path:%s, method:%s, perm:%d, mask:%d", username, path, method, perm, mask)
	if !ok || mask&perm != 0 {
		log.Printf("permission granted")
		return true
	}

	log.Printf("permission denied")
	return false
}

func (parent *ReposParent) start() {
	defer func() {
		err := recover()
		if err != nil {
			log.Printf("uncaught exr:%s", err)
			go parent.start()
		}
	}()

	for {
		time.Sleep(parent.refreshInterval)
		//初始化资源
		if parent.reposList == nil {
			parent.reposList = make(map[uint]*ReposRoot)
		}

		reposMap := parent.dao.getModifiedReposList(parent.lastReposCheckTime)
		for k, v := range reposMap {
			log.Printf("find new repos:%s", v.name)
			repos := NewReposRoot(k, v.name)
			pathList := parent.dao.getPathList(k)
			if v.lastModifyTime > parent.lastReposCheckTime {
				parent.lastReposCheckTime = v.lastModifyTime
			}
			for _, path := range pathList {
				log.Printf("add path:%s for repos:%s", path.path, v.name)
				repos.node.add(path.path, path.pid)
			}
			parent.reposList[k] = repos
		}

		//初始化用户
		if parent.users == nil {
			parent.users = make(map[string]*User)
		}

		modifiedUsers := parent.dao.getModifiedUser(parent.lastUserCheckTime)
		for _, userInfo := range modifiedUsers {
			user := new(User)
			user.uid = userInfo.uid
			user.uname = userInfo.uname
			log.Printf("find new user:%s", user.uname)
			user.perms = parent.dao.getPermPathList(user.uid)
			parent.users[user.uname] = user
			if userInfo.lastModifyTime > parent.lastUserCheckTime {
				parent.lastUserCheckTime = userInfo.lastModifyTime
			}
		}
		//log.Printf("check done")
	}
}

func (parent *ReposParent) getRid(name string) uint {
	for k, v := range parent.reposList {
		if v.name == name {
			return k
		}
	}
	return 0
}

type User struct {
	uid   uint
	uname string
	perms map[uint]uint
}

func newSvnAuther(refreshInterval time.Duration, dbhost string, ignoreList []string) SvnAuther {
	parent := new(ReposParent)
	parent.refreshInterval = refreshInterval
	parent.lastUserCheckTime = 0
	parent.lastReposCheckTime = 0
	parent.dao = NewSvnDao(dbhost)
	parent.ignoreList = ignoreList
	go parent.start()
	return parent
}

func (user *User) getPerm(pid map[uint]bool) uint {
	ret := uint(0)
	for k, _ := range pid {
		perm, ok := user.perms[k]
		if ok {
			ret |= perm
		}
	}

	return ret
}

func (root *PathNode) reset() {
	root.children = nil
}

/* vim: set ts=4 sw=4 sts=4 tw=100 noet: */
