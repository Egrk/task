package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cristalhq/acmd"
	"go.etcd.io/bbolt"
)

type TaskRecord struct {
	IsActive bool
	Description string
	FinishedAt time.Time
}

func getDb() *bbolt.DB {
	db, err := bbolt.Open("tasks.db", 0600, nil)
	if err != nil {
		panic(err)
	}
	return db
}

func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func btoi(v []byte) int {
	b, _ := binary.Uvarint(v)
	// if n != len(v) {
	// 	panic("len of converted int != len byte")
	// }
	return int(b)
}

var task = []byte("task")
var taskList [][]byte

func Add(ctx context.Context, cmdArgs []string) error {
	db := getDb()
	defer db.Close()
	err := db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(task)
		if err != nil {
				return err
		}

		id, _ := bucket.NextSequence()
		val := strings.Join(cmdArgs, " ")
		task := TaskRecord {
			IsActive: true,
			Description: val,
		}
		buf, err := json.Marshal(task)
		if err != nil {
			panic(err)
		}

		err = bucket.Put(itob(int(id)), buf)
		if err != nil {
				return err
		}
		return nil
	})
	if err != nil {
			panic(err)
	}
	return nil
}

func List(ctx context.Context, cmdArgs []string) error {
	db := getDb()
	defer db.Close()
	err := db.View(func(tx *bbolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(task))
		idx := 1
		taskList = nil
		b.ForEach(func(k, v []byte) error {
			var task TaskRecord
			err := json.Unmarshal(v, &task)
			if err != nil {
				panic(err)
			}
			if task.IsActive {
				fmt.Printf("%d. %s\n", idx, task.Description)
				idx++
				taskList = append(taskList, k)
			}

			return nil
		})

		return nil
	})
	if err != nil {
		panic(err)
	}
	return nil
}

func Do(ctx context.Context, cmdArgs []string) error {
	value, err := strconv.Atoi(cmdArgs[0])
	if err != nil {
		panic(err)
	}
	if len(taskList) == 0 {
		updateTaskList()
	}

	db := getDb()
	defer db.Close()
	err = db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(task)
		if bucket == nil {
				return fmt.Errorf("Bucket %q not found!", task)
		}

		val := bucket.Get(taskList[value-1])
		var task TaskRecord
		err := json.Unmarshal(val, &task)
		if err != nil {
			panic(err)
		}
		task.IsActive = false
		task.FinishedAt = time.Now()
		buf, err := json.Marshal(task)
		if err != nil {
			panic(err)
		}
		err = bucket.Put(taskList[value-1], buf)
		if err != nil {
			panic(err)
		}
		fmt.Printf("You have completed the %s task.", task.Description)

		return nil
	})
	if err != nil {
		panic(err)
	}

	return nil
}

func Remove(ctx context.Context, cmdArgs []string) error {
	value, err := strconv.Atoi(cmdArgs[0])
	if err != nil {
		panic(err)
	}
	if len(taskList) == 0 {
		updateTaskList()
	}
	db := getDb()
	defer db.Close()
	err = db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(task)
		if bucket == nil {
				return fmt.Errorf("Bucket %q not found!", task)
		}
		return bucket.Delete(taskList[value-1])
	})
	if err != nil {
		panic(err)
	}

	return nil
}

func Completed(ctx context.Context, cmdArgs []string) error {
	db := getDb()
	defer db.Close()
	err := db.View(func(tx *bbolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(task))
		fmt.Println("This task(s) was completed today:")
		b.ForEach(func(k, v []byte) error {
			var task TaskRecord
			err := json.Unmarshal(v, &task)
			if err != nil {
				panic(err)
			}
			if !task.IsActive && time.Now().Day() - task.FinishedAt.Day() <= 0 {
				fmt.Printf("%s\n", task.Description)
			}

			return nil
		})

		return nil
	})
	if err != nil {
		panic(err)
	}
	return nil
}

func updateTaskList() {
	taskList = nil
	db := getDb()
	defer db.Close()
	err := db.View(func(tx *bbolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(task))
		b.ForEach(func(k, v []byte) error {
			var task TaskRecord
			err := json.Unmarshal(v, &task)
			if err != nil {
				panic(err)
			}
			if task.IsActive {
				taskList = append(taskList, k)
			}
			return nil
		})
		return nil
	})
	if err != nil {
		panic(err)
	}
}

func main() {

	cmds := []acmd.Command{
		{
			Name:        "add",
			Description: "Add task",
			ExecFunc:    Add,
		},
		{
			Name:        "list",
			Description: "List all active tasks",
			ExecFunc:    List,
		},
		{
			Name:     "do",
			Description: "Mark task as done",
			ExecFunc: Do,
		},
		{
			Name:     "remove",
			Description: "Delete task",
			ExecFunc: Remove,
		},
		{
			Name:     "completed",
			Description: "Show today completed tasks",
			ExecFunc: Completed,
		},
	}

	r := acmd.RunnerOf(cmds, acmd.Config{
		AppName:         "acmd-example",
		AppDescription:  "Example of acmd package",
		PostDescription: "Best place to add examples.",
		Version:         "the best v0.x.y",
	})

	if err := r.Run(); err != nil {
		panic(err)
	}
}