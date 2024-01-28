package main

import (
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/labring/sealos/controllers/pkg/database/cockroach"

	"github.com/gin-gonic/gin"
)

var GROUP = "/account/v1alpha1"

type InviteReward struct {
	UID   string   `json:"uid"`
	Users []string `json:"users"`
}

var inviteMutexMap = make(map[string]*sync.Mutex)

func main() {
	r := gin.Default()

	r.Use(authenticateMiddleware())

	ck, err := cockroach.NewCockRoach("", "")
	if err != nil {
		fmt.Printf("failed to new CockRoach DB: %v", err)
		os.Exit(1)
	}
	r.Group(GROUP).
		POST("/handlerReward", func(c *gin.Context) {
			/*
				{
				  uid: '94313c2b-8be1-4871-b1cf-ef8284f2ecb1',
				  users: [
				    '42865e77-eed0-4fe7-a459-671803cad448',
				    '6caf86f2-331f-41fb-875f-addf8876d026',
				    'acaedac5-e552-4856-9554-a1a0cd2ea603',
				    '40579dfd-4a87-4e8b-82d6-52ec991d9a4d'
				  ]
				}
			*/

			inviteRewardReq, err := BindInviteReward(c)
			if err != nil {
				fmt.Printf("bind json error : %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("bind json error : %v", err)})
				return
			}
			var tx *sync.Mutex
			var ok bool
			tx, ok = inviteMutexMap[inviteRewardReq.UID]

			if !ok {
				inviteMutexMap[inviteRewardReq.UID] = &sync.Mutex{}
			}
			if !tx.TryLock() {
				fmt.Printf("try lock error : %v", err)
				c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("try lock error : %v", err)})
				return
			}
			defer tx.Unlock()

			amount, err := ck.InviteRewardHandler(inviteRewardReq.UID, inviteRewardReq.Users, 0.1)
			if err != nil {
				fmt.Printf("failed to invite reward handler: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to invite reward handler: %v", err)})
			}
			c.JSON(http.StatusOK, gin.H{"amount": amount})
		}).
		POST("/getReward", func(c *gin.Context) {
			/*
				{
				  uid: '94313c2b-8be1-4871-b1cf-ef8284f2ecb1',
				  users: [
				    '42865e77-eed0-4fe7-a459-671803cad448',
				    '6caf86f2-331f-41fb-875f-addf8876d026',
				    'acaedac5-e552-4856-9554-a1a0cd2ea603',
				    '40579dfd-4a87-4e8b-82d6-52ec991d9a4d'
				  ]
				}
			*/
			inviteRewardReq, err := BindInviteReward(c)
			if err != nil {
				fmt.Printf("bind json error : %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("bind json error : %v", err)})
				return
			}
			owner := inviteRewardReq.UID

			rewardList, err := ck.GetInviteReward(owner)
			if err != nil {
				fmt.Printf("get reward list error : %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("get reward list error : %v", err)})
				return
			}
			fmt.Printf("owner: %s, rewardList: %v\n", owner, rewardList)
			c.JSON(http.StatusOK, gin.H{"rewardList": rewardList})
		})
	if err := r.Run(":2334"); err != nil {
		fmt.Printf("failed to license addr: %v", err)
		os.Exit(1)
	}
}

func BindInviteReward(c *gin.Context) (*InviteReward, error) {
	newInviteReward := &InviteReward{}
	err := c.BindJSON(newInviteReward)
	if err != nil {
		return nil, fmt.Errorf("bind json error : %v", err)
	}
	if newInviteReward.UID == "" {
		return nil, fmt.Errorf("uid is required")
	}
	if len(newInviteReward.Users) == 0 {
		return nil, fmt.Errorf("users is required")
	}
	return newInviteReward, nil
}

const secretToken = "thXKmZXKSwX22TkB9Fonx"

func authenticateMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token != "Bearer "+secretToken {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
	}
}
