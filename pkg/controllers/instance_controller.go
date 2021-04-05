/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"math/rand"
	"time"

	"github.com/google/uuid"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1beta1 "github.com/julienstroheker/kube-pokeapi-arena/pkg/api/v1beta1"
	"github.com/julienstroheker/kube-pokeapi-arena/pkg/random"
)

const requeueAfter = 30 * time.Second

// InstanceReconciler reconciles a Instance object
type InstanceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.kube-pokeapi-arena.io,resources=instances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.kube-pokeapi-arena.io,resources=instances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
func (r *InstanceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("instance", req.NamespacedName)

	var instance corev1beta1.Instance
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		log.Error(err, "unable to fetch Instance")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// On Schedule every 30sec
	// Check status of living instance and compare with Max

	// Randomize between living and max
	// Spawn the randomized pokemon
	// Reschedule for 30sec
	var pokemonList corev1.PodList
	err := r.List(ctx, &pokemonList, client.InNamespace(req.Namespace))
	if err != nil {
		log.Error(err, "unable to list pokemons")
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, client.IgnoreNotFound(err)
	}

	switch missingPokemons := int(instance.Spec.Settings.MaxPokemons) - len(pokemonList.Items); {
	// WIll try to spwan more pokemons
	case missingPokemons > 0:
		var src random.Source
		rnd := rand.New(src)
		randomSpawn := rnd.Intn(missingPokemons + 1)
		log.Info("Random magic number is now", "number", randomSpawn)
		for i := 0; i < randomSpawn; i++ {
			aNewPokemon := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pokemon-" + uuid.New().String(),
					Namespace: req.Namespace,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "pokemon",
							Image: "nginx:1.12",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									Protocol:      corev1.ProtocolTCP,
									ContainerPort: 80,
								},
							},
						},
					},
				},
			}
			err := r.Create(ctx, &aNewPokemon)
			if err != nil {
				log.Error(err, "unable to create pokemon")
				return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, client.IgnoreNotFound(err)
			}
			instance.Status.Living = uint64(len(pokemonList.Items)) + 1
			instance.Status.Spawn = instance.Status.Spawn + 1
			err = r.Update(ctx, &instance)
			if err != nil {
				return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, err
			}
		}
	// Need to remove some pokemons
	case missingPokemons < 0:
		var src random.Source
		rnd := rand.New(src)
		// Pod to delete
		podNumberToDelete := rnd.Intn(len(pokemonList.Items) + 1)
		log.Info("Random magic number is now", "number", podNumberToDelete)
		podToDelete := pokemonList.Items[podNumberToDelete]
		err = r.Delete(ctx, &podToDelete)
		if err != nil {
			return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, err
		}
	default:
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}

	return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
}

func (r *InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1beta1.Instance{}).
		WithEventFilter(predicate.Funcs{
			DeleteFunc: func(e event.DeleteEvent) bool {
				// Suppress Delete events to avoid filtering them out in the Reconcile function
				return false
			},
		}).
		Complete(r)
}
