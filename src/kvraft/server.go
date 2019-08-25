package raftkv

import (
	"labgob"
	"labrpc"
	"log"
	"raft"
	"sync"
	"time"
)

const Debug = 1

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}


type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	Key string
	Value string
	Type string
}

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	keyValue map[string] string
	getCh map[int]chan Op
}

func (kv *KVServer) getAgreeCh(index int) chan Op{
	kv.mu.Lock()
	defer kv.mu.Unlock()
	ch, ok := kv.getCh[index]
	if !ok{
		ch = make(chan Op, 1)
		kv.getCh[index] = ch
	//	DPrintf("init ch %v\n", index)
	}
	return ch
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
	index, _, isLeader := kv.rf.Start(Op{args.Key, "", "Get"})
	reply.WrongLeader = false
	if !isLeader {
		reply.WrongLeader = true
	} else {
		ch := kv.getAgreeCh(index)
		select {
		case <-ch:
			close(ch)
			reply.Value = kv.keyValue[args.Key]
		//	DPrintf("%v agree\n", kv.me)
			return
		case <-time.After(1000*time.Millisecond):
			reply.WrongLeader = true
			return
		}
	}
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	index, _, isLeader := kv.rf.Start(Op{args.Key, args.Value, args.Op})
	reply.WrongLeader = false
	if !isLeader {
		reply.WrongLeader = true
	} else {
		ch := kv.getAgreeCh(index)
	//	DPrintf("%v is leader\n", kv.me)
		select {
		case <- ch:
			close(ch)
		//	DPrintf("%v agree\n", kv.me)
			return
		case <-time.After(1000*time.Millisecond):
			reply.WrongLeader = true
			return
		}
	}
}

//
// the tester calls Kill() when a KVServer instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (kv *KVServer) Kill() {
	kv.rf.Kill()
	// Your code here, if desired.
}
func (kv *KVServer) waitSubmitLoop() {
	for {
		select {
		case msg := <-kv.applyCh:
			op := msg.Command.(Op)
		//	DPrintf("%v %v key:%v, value:%v\n", kv.me, op.Type, op.Key, op.Value)
			kv.mu.Lock()
			switch op.Type {
			case "Put":
				kv.keyValue[op.Key] = op.Value
			case "Append":
				kv.keyValue[op.Key] += op.Value
			}
			kv.mu.Unlock()
/*			for k, v := range kv.keyValue {
				DPrintf("%v's key:%v, value:%v\n", kv.me, k, v)
			}*/
			kv.getAgreeCh(msg.CommandIndex) <- op
		}
	}
}
//
// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
//
func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	// You may need initialization code here.
	kv.getCh = make(map[int]chan Op)
	kv.keyValue = make(map[string] string)
	go kv.waitSubmitLoop()

	return kv
}