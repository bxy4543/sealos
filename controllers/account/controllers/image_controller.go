package controllers

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	ntfv1 "github.com/labring/sealos/controllers/pkg/notification/api/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// ImageReconciler 用于协调 Deployment 和 StatefulSet 资源
type ImageReconciler struct {
	Account *AccountReconciler
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	//Cache  cache.Cache
	Domain         string
	NamespaceCache *sync.Map
}

func (r *ImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lock, has := r.NamespaceCache.LoadOrStore(req.Namespace, &sync.Mutex{})
	if !has {
		lock = &sync.Mutex{}
		r.NamespaceCache.Store(req.Namespace, lock)
	}
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()
	log := r.Log.WithValues("resource", req.NamespacedName)

	// 使用通用方法获取资源
	obj, err := r.getResource(ctx, req)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "无法获取资源")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	containers, meta := r.extractResourceInfo(obj)

	return r.handleResource(ctx, containers, meta, log)
}

func (r *ImageReconciler) getResource(ctx context.Context, req ctrl.Request) (client.Object, error) {
	deploy := &appsv1.Deployment{}
	err := r.Get(ctx, req.NamespacedName, deploy)
	if err == nil {
		return deploy, nil
	}
	if !errors.IsNotFound(err) {
		return nil, err
	}

	sts := &appsv1.StatefulSet{}
	err = r.Get(ctx, req.NamespacedName, sts)
	if err != nil {
		return nil, err
	}
	return sts, nil
}

func (r *ImageReconciler) extractResourceInfo(obj client.Object) ([]v1.Container, v12.ObjectMeta) {
	switch v := obj.(type) {
	case *appsv1.Deployment:
		return v.Spec.Template.Spec.Containers, v.ObjectMeta
	case *appsv1.StatefulSet:
		return v.Spec.Template.Spec.Containers, v.ObjectMeta
	default:
		return nil, v12.ObjectMeta{}
	}
}

func (r *ImageReconciler) handleResource(_ context.Context, containers []v1.Container, meta v12.ObjectMeta, log logr.Logger) (ctrl.Result, error) {
	for _, container := range containers {
		if strings.Contains(container.Image, "hub."+r.Domain) {
			isSet, err := r.Account.AccountV2.SetAccountDevbox1024Transaction(meta.Namespace)
			if err != nil {
				log.Error(err, "设置 devbox 交易失败", "namespace", meta.Namespace, "name", meta.Name)
				return ctrl.Result{}, err
			}
			if isSet {
				// TODO 信息通知
				now := time.Now().UTC().Unix()
				ntfTmp := &ntfv1.Notification{
					ObjectMeta: v12.ObjectMeta{
						Name:      "devbox-transaction-" + strconv.FormatInt(now, 10),
						Namespace: meta.Namespace,
					},
				}
				ntfTmpSpc := ntfv1.NotificationSpec{
					Title:        "devbox 1024 activity",
					Message:      "you have successfully participated in the devbox 1024 event, and 20 balance has been recharged for you.",
					From:         "Active-System",
					Importance:   "High",
					DesktopPopup: true,
					Timestamp:    now,
					I18n: map[string]ntfv1.I18n{
						languageZh: {
							Title:   "活动通知",
							From:    "费用中心",
							Message: "您已成功参与 devbox 1024 活动，已为您充值 20 额度。",
						},
					},
				}
				if ntfTmp.Labels == nil {
					ntfTmp.Labels = make(map[string]string)
				}
				ntfTmp.Labels[readStatusLabel] = "false"
				ntfTmp.Spec = ntfTmpSpc
				err = r.Create(context.Background(), ntfTmp)
				if err != nil {
					log.Error(err, "创建通知失败", "namespace", meta.Namespace, "name", meta.Name)
				}
			}
			return ctrl.Result{}, nil
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager 设置 controller 与 Manager
func (r *ImageReconciler) SetupWithManager(mgr ctrl.Manager, rateOpts controller.Options) error {
	r.Domain = os.Getenv("DOMAIN")
	r.Log = ctrl.Log.WithName("controllers").WithName("Image")
	r.NamespaceCache = &sync.Map{}
	//// 创建一个新的缓存，只关注 spec 字段
	//_cache, err := cache.New(mgr.GetConfig(), cache.Options{
	//	Scheme: mgr.GetScheme(),
	//	Mapper: mgr.GetRESTMapper(),
	//	ByObject: map[client.Object]cache.ByObject{
	//		&appsv1.Deployment{}: {
	//			Field: fields.SelectorFromSet(fields.Set{"spec": ""}),
	//		},
	//		&appsv1.StatefulSet{}: {
	//			Field: fields.SelectorFromSet(fields.Set{"spec": ""}),
	//		},
	//	},
	//})
	//if err != nil {
	//	return err
	//}
	//r.Cache = _cache

	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}, builder.WithPredicates(OnlyCreatePredicate{})).
		Watches(&appsv1.StatefulSet{}, handler.EventHandler(OnlyCreateHandler{})).
		WithOptions(rateOpts).
		Complete(r)
}

type OnlyCreateHandler struct {
}

// Create 处理创建事件
func (o OnlyCreateHandler) Create(_ context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	// 将创建事件的对象添加到工作队列中
	q.Add(reconcile.Request{NamespacedName: client.ObjectKeyFromObject(evt.Object)})
}

// Update 处理更新事件（空实现）
func (o OnlyCreateHandler) Update(_ context.Context, _ event.UpdateEvent, _ workqueue.RateLimitingInterface) {
	// 不处理更新事件
}

// Delete 处理删除事件（空实现）
func (o OnlyCreateHandler) Delete(_ context.Context, _ event.DeleteEvent, _ workqueue.RateLimitingInterface) {
	// 不处理删除事件
}

// Generic 处理通用事件（空实现）
func (o OnlyCreateHandler) Generic(_ context.Context, _ event.GenericEvent, _ workqueue.RateLimitingInterface) {
	// 不处理通用事件
}
