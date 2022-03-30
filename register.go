package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
)

const (
	_ttl = 10
)

type Register struct {
	cli       *clientv3.Client
	leaseId   clientv3.LeaseID
	lease     clientv3.Lease
	info      *NodeInfo
	closeChan chan error
}

func NewRegister(info *NodeInfo, conf clientv3.Config) (reg *Register, err error) {
	r := &Register{}
	r.closeChan = make(chan error)
	r.info = info
	r.cli, err = clientv3.New(conf)
	return r, err
}

func (r *Register) Run() {
	dur := time.Duration(time.Second)
	timer := time.NewTicker(dur)
	r.register()
	for {
		select {
		case <-timer.C:
			r.keepAlive()
		case <-r.closeChan:
			goto EXIT
		}
	}
EXIT:
	log.Printf("[Register] Run exit...")
}

func (r *Register) Stop() {
	r.revoke()
	close(r.closeChan)
}

func (r *Register) register() (err error) {
	r.leaseId = 0
	kv := clientv3.NewKV(r.cli)
	r.lease = clientv3.NewLease(r.cli)
	leaseResp, err := r.lease.Grant(context.TODO(), _ttl)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("ttl:", leaseResp.TTL)
	data, err := json.Marshal(r.info)
	_, err = kv.Put(context.TODO(), r.info.UniqueId, string(data), clientv3.WithLease(leaseResp.ID))
	if err != nil {
		fmt.Printf(err.Error(), "[Register] register kv.Put err %s-%+v", r.info.Name, string(data))
		return
	}
	r.leaseId = leaseResp.ID
	return
}

//上行心跳保活
func (r *Register) keepAlive() (err error) {
	_, err = r.lease.KeepAliveOnce(context.TODO(), r.leaseId)
	if err != nil {
		// 租约丢失，重新注册
		if err == rpctypes.ErrLeaseNotFound {
			r.register()
			err = nil
		}
		fmt.Println(err.Error())
	}
	log.Printf(fmt.Sprintf("[Register] keepalive... leaseId:%+v", r.leaseId))
	return err
}

//撤销租约
func (r *Register) revoke() (err error) {
	_, err = r.cli.Revoke(context.TODO(), r.leaseId)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	log.Printf(fmt.Sprintf("[Register] revoke node:%+v", r.leaseId))
	return
}
