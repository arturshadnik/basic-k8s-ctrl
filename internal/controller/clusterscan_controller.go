/*
Copyright 2024 arturshadnik.

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

package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	scanv1 "basic-k8s-ctrl/api/v1"
)

// ClusterScanReconciler reconciles a ClusterScan object
type ClusterScanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=scan.arturshadnik.io,resources=clusterscans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=scan.arturshadnik.io,resources=clusterscans/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=scan.arturshadnik.io,resources=clusterscans/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterScan object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile

func (r *ClusterScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	clusterScan := &scanv1.ClusterScan{}

	err := r.Get(ctx, req.NamespacedName, clusterScan)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	jobName := fmt.Sprintf("%s-job", clusterScan.Name)
	jobNamespace := clusterScan.Namespace

	jobMeta := metav1.ObjectMeta{
		Name:      jobName,
		Namespace: jobNamespace,
		Labels:    map[string]string{"clusterscan": clusterScan.Name},
	}

	if clusterScan.Spec.OneOff {
		err = r.reconcileJob(ctx, clusterScan, jobMeta, l)
		if err != nil {
			return ctrl.Result{}, err
		}

	} else {
		err = r.reconcileCronJob(ctx, clusterScan, jobMeta, l)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// Helpers
func (r *ClusterScanReconciler) reconcileJob(ctx context.Context, scan *scanv1.ClusterScan, meta metav1.ObjectMeta, l logr.Logger) error {
	job := &batchv1.Job{}

	err := r.Get(ctx, client.ObjectKey{Name: meta.Name, Namespace: meta.Namespace}, job)
	if err != nil && client.IgnoreNotFound(err) != nil { // case where job cannot be retrieved for  any reason
		return err

	} else if err != nil { // case where job does not exist
		l.Info("Creating one-off job")

		err = r.submitJob(ctx, scan, meta, *job)
		if err != nil {
			l.Error(err, "Failed to submit job")
			return err
		}

		err := r.updateClusterScanStatus(ctx, scan, job)
		if err != nil {
			l.Error(err, "Failed to update ClusterScan status")
			return err
		}

	} else { // job status updated
		err = r.updateClusterScanStatus(ctx, scan, job)
		if err != nil {
			l.Error(err, "Failed to update ClusterScan status")
			return err
		}
	}
	return nil
}

func (r *ClusterScanReconciler) reconcileCronJob(ctx context.Context, scan *scanv1.ClusterScan, meta metav1.ObjectMeta, l logr.Logger) error {
	job := &batchv1.CronJob{}
	err := r.Get(ctx, client.ObjectKey{Name: meta.Name, Namespace: meta.Namespace}, job)
	if err != nil && client.IgnoreNotFound(err) != nil { // case where job cannot be retrieved for  any reason
		return err

	} else if err != nil { // case where cron hasnt been created
		l.Info("Scheduling cron job")

		err = r.submitCronJob(ctx, scan, meta, *job)
		if err != nil {
			return err
		}

		scan.Status.Phase = "Scheduled"
		scan.Status.StartTime = metav1.Now()
		err = r.Status().Update(ctx, scan)
		if err != nil {
			log.Log.Error(err, "Failed to update ClusterScan status")
			return err
		}
	}

	jobList := &batchv1.JobList{}
	labelSelector := client.MatchingLabels{"clusterscan": scan.Name}

	err = r.Client.List(ctx, jobList, labelSelector)
	if err != nil {
		log.Log.Error(err, "Failed to update ClusterScan status")
		return err
	}

	filteredJobs := []*batchv1.Job{}
	// filter jobs that are already accounted for, using CompletionTime
	for _, job := range jobList.Items {
		if job.Status.CompletionTime != nil && !job.Status.CompletionTime.Time.After(scan.Status.LastExecutionDetails.CompletionTime.Time) {
			filteredJobs = append(filteredJobs, &job)
		}
	}
	for _, job := range filteredJobs {
		err = r.updateClusterScanStatus(ctx, scan, job)
		if err != nil {
			log.Log.Error(err, "Failed to update ClusterScan status")
			return err
		}
	}
	return nil
}

func (r *ClusterScanReconciler) submitJob(ctx context.Context, scan *scanv1.ClusterScan, meta metav1.ObjectMeta, job batchv1.Job) error {
	job = batchv1.Job{
		ObjectMeta: meta,
		Spec: batchv1.JobSpec{
			Template:     r.createPodTemplate(scan),
			BackoffLimit: ptr.To(int32(1)),
		},
	}

	err := r.Create(ctx, &job)
	if err != nil {
		return err
	}
	if err := controllerutil.SetControllerReference(scan, &job, r.Scheme); err != nil {
		return err
	}
	return nil
}

func (r *ClusterScanReconciler) submitCronJob(ctx context.Context, scan *scanv1.ClusterScan, meta metav1.ObjectMeta, job batchv1.CronJob) error {
	job = batchv1.CronJob{
		ObjectMeta: meta,
		Spec: batchv1.CronJobSpec{
			Schedule: scan.Spec.Schedule,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: r.createPodTemplate(scan),
				},
			},
		},
	}
	err := r.Create(ctx, &job)
	if err != nil {
		return err
	}
	if err := controllerutil.SetControllerReference(scan, &job, r.Scheme); err != nil {
		return err
	}
	return nil
}

func (r *ClusterScanReconciler) createPodTemplate(scan *scanv1.ClusterScan) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"clusterscan": scan.Name},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "scanner",
					Image:   scan.Spec.Image,
					Command: scan.Spec.Command,
					Args:    scan.Spec.Args,
				},
			},
			RestartPolicy: corev1.RestartPolicy(scan.Spec.RestartPolicy),
		},
	}
}

func (r *ClusterScanReconciler) updateClusterScanStatus(ctx context.Context, scan *scanv1.ClusterScan, job *batchv1.Job) error {
	if len(job.Status.Conditions) == 0 { // initial phase
		scan.Status.StartTime = metav1.Now()
		scan.Status.Phase = "Running"

	} else {
		c := job.Status.Conditions[len(job.Status.Conditions)-1]
		if batchv1.JobConditionType(c.Type) == batchv1.JobComplete && corev1.ConditionStatus(c.Status) == corev1.ConditionTrue {
			scan.Status.Phase = "Succeeded"
			scan.Status.CompletionTime = metav1.Now()
			scan.Status.Succeeded += int(job.Status.Succeeded)
			scan.Status.Failed += int(job.Status.Failed)
			if job.Status.CompletionTime.Time.After(scan.Status.CompletionTime.Time) {
				// only update when the job happened after the last recorded one
				scan.Status.LastExecutionDetails = scanv1.ExecutionDetails{
					StartTime:      *job.Status.StartTime,
					CompletionTime: c.LastTransitionTime,
					Result:         "Completed successfully",
				}
			}
			scan.Status.Conditions = append(scan.Status.Conditions, c)

		} else if batchv1.JobConditionType(c.Type) == batchv1.JobFailed && corev1.ConditionStatus(c.Status) == corev1.ConditionTrue {
			scan.Status.Phase = "Failed"
			scan.Status.CompletionTime = metav1.Now()
			scan.Status.Succeeded = int(job.Status.Succeeded)
			scan.Status.Failed = int(job.Status.Failed)
			if job.Status.CompletionTime.Time.After(scan.Status.CompletionTime.Time) {
				scan.Status.LastExecutionDetails = scanv1.ExecutionDetails{
					StartTime:      *job.Status.StartTime,
					CompletionTime: c.LastTransitionTime,
					Result:         "Execution failed",
				}
				scan.Status.Conditions = append(scan.Status.Conditions, c)
			}
		}
	}
	return r.Status().Update(ctx, scan)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scanv1.ClusterScan{}).
		Owns(&batchv1.CronJob{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
