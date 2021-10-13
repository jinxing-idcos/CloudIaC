// Copyright 2021 CloudJ Company Limited. All rights reserved.

package models

import (
	"cloudiac/common"
	"cloudiac/portal/libs/db"
	"cloudiac/runner"
	"cloudiac/utils"
	"database/sql/driver"
	"fmt"
	"path"
)

type TaskVariables []VariableBody

func (v TaskVariables) Value() (driver.Value, error) {
	return MarshalValue(v)
}

func (v *TaskVariables) Scan(value interface{}) error {
	return UnmarshalValue(value, v)
}

type TaskResult struct {
	ResAdded     *int `json:"resAdded"` // 该值为 nil 表示无资源变更数据(区别于 0)
	ResChanged   *int `json:"resChanged"`
	ResDestroyed *int `json:"resDestroyed"`

	Outputs map[string]interface{} `json:"outputs"`
}

func (v TaskResult) Value() (driver.Value, error) {
	return MarshalValue(v)
}

func (v *TaskResult) Scan(value interface{}) error {
	return UnmarshalValue(value, v)
}

type TaskExtra struct {
	Source       string `json:"source,omitempty"`
	TransitionId string `json:"transitionId,omitempty"`
}

func (v TaskExtra) Value() (driver.Value, error) {
	return MarshalValue(v)
}

func (v *TaskExtra) Scan(value interface{}) error {
	return UnmarshalValue(value, v)
}

const (
	TaskTypePlan    = common.TaskTypePlan
	TaskTypeApply   = common.TaskTypeApply
	TaskTypeDestroy = common.TaskTypeDestroy
	TaskTypeScan    = common.TaskTypeScan
	TaskTypeParse   = common.TaskTypeParse

	TaskPending   = common.TaskPending
	TaskRunning   = common.TaskRunning
	TaskApproving = common.TaskApproving
	TaskRejected  = common.TaskRejected
	TaskFailed    = common.TaskFailed
	TaskComplete  = common.TaskComplete
)

type Tasker interface {
	GetId() Id
	GetRunnerId() string
	GetStepTimeout() int
	Exited() bool
	Started() bool
	IsStartedStatus(status string) bool
	IsExitedStatus(status string) bool
	IsEffectTask() bool
	IsEffectTaskType(typ string) bool
	GetTaskNameByType(typ string) string
}

// Task 部署任务
type Task struct {
	BaseTask

	OrgId     Id `json:"orgId" gorm:"size:32;not null"`     // 组织ID
	ProjectId Id `json:"projectId" gorm:"size:32;not null"` // 项目ID
	TplId     Id `json:"tplId" gorm:"size:32;not null"`     // 模板ID
	EnvId     Id `json:"envId" gorm:"size:32;not null"`     // 环境ID

	Name      string `json:"name" gorm:"not null;comment:任务名称"` // 任务名称
	CreatorId Id     `json:"creatorId" gorm:"size:32;not null"` // 创建人ID

	RepoAddr string `json:"repoAddr" gorm:"not null"`
	Revision string `json:"revision" gorm:"not null"`
	CommitId string `json:"commitId" gorm:"not null"` // 创建任务时 revision 对应的 commit id

	Workdir      string   `json:"workdir" gorm:"default:''"`
	Playbook     string   `json:"playbook" gorm:"default:''"`
	TfVarsFile   string   `json:"tfVarsFile" gorm:"default:''"`
	TfVersion    string   `json:"tfVersion" gorm:"default:''"`
	PlayVarsFile string   `json:"playVarsFile" gorm:"default:''"`
	Targets      StrSlice `json:"targets" gorm:"type:json"` // 指定 terraform target 参数

	Variables TaskVariables `json:"variables" gorm:"type:json"` // 本次执行使用的所有变量(继承、覆盖计算之后的)

	StatePath string `json:"statePath" gorm:"not null"`

	// 扩展属性，包括 source, transitionId 等
	Extra TaskExtra `json:"extra" gorm:"type:json"` // 扩展属性

	KeyId           Id   `json:"keyId" gorm:"size32"` // 部署密钥ID
	AutoApprove     bool `json:"autoApproval" gorm:"default:false"`
	StopOnViolation bool `json:"stopOnViolation" gorm:"default:false"`

	// 任务执行结果，如 add/change/delete 的资源数量、outputs 等
	Result TaskResult `json:"result" gorm:"type:json"` // 任务执行结果

	RetryNumber int  `json:"retryNumber" gorm:"size:32;default:0"` // 任务重试次数
	RetryDelay  int  `json:"retryDelay" gorm:"size:32;default:0"`  // 每次任务重试时间，单位为秒
	RetryAble   bool `json:"retryAble" gorm:"default:false"`
}

func (Task) TableName() string {
	return "iac_task"
}

func (Task) DefaultTaskName() string {
	return ""
}

func (t *BaseTask) GetId() Id {
	return t.Id
}

func (t *BaseTask) GetRunnerId() string {
	return t.RunnerId
}

func (t *BaseTask) GetStepTimeout() int {
	return t.StepTimeout
}

func (t *BaseTask) Exited() bool {
	return t.IsExitedStatus(t.Status)
}

func (t *BaseTask) Started() bool {
	return t.IsStartedStatus(t.Status)
}

func (BaseTask) IsStartedStatus(status string) bool {
	// 注意：approving 状态的任务我们也认为其 started
	return !utils.InArrayStr([]string{TaskPending}, status)
}

func (BaseTask) IsExitedStatus(status string) bool {
	return utils.InArrayStr([]string{TaskFailed, TaskRejected, TaskComplete}, status)
}

func (t *BaseTask) IsEffectTask() bool {
	return t.IsEffectTaskType(t.Type)
}

// IsEffectTaskType 是否产生实际数据变动的任务类型
func (BaseTask) IsEffectTaskType(typ string) bool {
	return utils.StrInArray(typ, TaskTypeApply, TaskTypeDestroy)
}

func (BaseTask) GetTaskNameByType(typ string) string {
	switch typ {
	case TaskTypePlan:
		return common.TaskTypePlanName
	case TaskTypeApply:
		return common.TaskTypeApplyName
	case TaskTypeDestroy:
		return common.TaskTypeDestroyName
	case TaskTypeScan:
		return common.TaskTypeScanName
	case TaskTypeParse:
		return common.TaskTypeParse
	default:
		panic("invalid task type")
	}
}
func (t *Task) StateJsonPath() string {
	return path.Join(t.ProjectId.String(), t.EnvId.String(), t.Id.String(), runner.TFStateJsonFile)
}

func (t *Task) ProviderSchemaJsonPath() string {
	return path.Join(t.ProjectId.String(), t.EnvId.String(), t.Id.String(), runner.TFProviderSchema)
}

func (t *Task) PlanJsonPath() string {
	return path.Join(t.ProjectId.String(), t.EnvId.String(), t.Id.String(), runner.TFPlanJsonFile)
}

func (t *Task) TfParseJsonPath() string {
	return path.Join(t.ProjectId.String(), t.EnvId.String(), t.Id.String(), runner.TerrascanJsonFile)
}

func (t *Task) TfResultJsonPath() string {
	return path.Join(t.ProjectId.String(), t.EnvId.String(), t.Id.String(), runner.TerrascanResultFile)
}

func (t *Task) HideSensitiveVariable() {
	for index, v := range t.Variables {
		if v.Sensitive {
			t.Variables[index].Value = ""
		}
	}
}

func (t *Task) Migrate(sess *db.Session) (err error) {
	if err := sess.ModifyModelColumn(t, "status"); err != nil {
		return err
	}
	return nil
}

type TaskStepBody struct {
	Type string   `json:"type" yaml:"type" gorm:"type:enum('init','plan','apply','play','command','destroy','scaninit','tfscan','tfparse','scan')"`
	Name string   `json:"name,omitempty" yaml:"name" gorm:"size:32;not null"`
	Args StrSlice `json:"args,omitempty" yaml:"args" gorm:"type:text"`
}

const (
	TaskStepInit     = common.TaskStepInit
	TaskStepPlan     = common.TaskStepPlan
	TaskStepApply    = common.TaskStepApply
	TaskStepDestroy  = common.TaskStepDestroy
	TaskStepPlay     = common.TaskStepPlay
	TaskStepCommand  = common.TaskStepCommand
	TaskStepCollect  = common.TaskStepCollect
	TaskStepTfParse  = common.TaskStepTfParse
	TaskStepTfScan   = common.TaskStepTfScan
	TaskStepScanInit = common.TaskStepScanInit

	TaskStepPending   = common.TaskStepPending
	TaskStepApproving = common.TaskStepApproving
	TaskStepRejected  = common.TaskStepRejected
	TaskStepRunning   = common.TaskStepRunning
	TaskStepFailed    = common.TaskStepFailed
	TaskStepComplete  = common.TaskStepComplete
	TaskStepTimeout   = common.TaskStepTimeout
)

type TaskStep struct {
	BaseModel
	TaskStepBody

	OrgId     Id     `json:"orgId" gorm:"size:32;not null"`
	ProjectId Id     `json:"projectId" gorm:"size:32;not null"`
	EnvId     Id     `json:"envId" gorm:"size:32;not null"`
	TaskId    Id     `json:"taskId" gorm:"size:32;not null"`
	NextStep  Id     `json:"nextStep" gorm:"size:32;default:''"`
	Index     int    `json:"index" gorm:"size:32;not null"`
	Status    string `json:"status" gorm:"type:enum('pending','approving','rejected','running','failed','complete','timeout')"`
	ExitCode  int    `json:"exitCode" gorm:"default:0"` // 执行退出码，status 为 failed 时才有意义
	Message   string `json:"message" gorm:"type:text"`
	StartAt   *Time  `json:"startAt" gorm:"type:datetime"`
	EndAt     *Time  `json:"endAt" gorm:"type:datetime"`
	LogPath   string `json:"logPath" gorm:""`

	ApproverId Id `json:"approverId" gorm:"size:32;not null"` // 审批者用户 id

	CurrentRetryCount int   `json:"currentRetryCount" gorm:"size:32;default:0"` // 当前重试次数
	NextRetryTime     int64 `json:"nextRetryTime" gorm:"default:0"`             // 下次重试时间
	RetryNumber       int   `json:"retryNumber" gorm:"size:32;default:0"`       // 每个步骤可以重试的总次数
}

func (TaskStep) TableName() string {
	return "iac_task_step"
}

func (t *TaskStep) Migrate(sess *db.Session) (err error) {
	if err := sess.ModifyModelColumn(t, "type"); err != nil {
		return err
	}
	return nil
}

func (s *TaskStep) IsStarted() bool {
	return !utils.StrInArray(s.Status, TaskStepPending, TaskStepApproving)
}

func (s *TaskStep) IsExited() bool {
	return utils.StrInArray(s.Status, TaskStepRejected, TaskStepComplete, TaskStepFailed, TaskStepTimeout)
}

func (s *TaskStep) IsApproved() bool {
	if s.Status == TaskStepRejected {
		return false
	}
	// 只有 apply 和 destroy 步骤需要审批
	if utils.StrInArray(s.Type, TaskStepApply, TaskStepDestroy) && len(s.ApproverId) == 0 {
		return false
	}
	return true
}

func (s *TaskStep) IsRejected() bool {
	return s.Status == TaskStepRejected
}

func (s *TaskStep) GenLogPath() string {
	return path.Join(
		s.ProjectId.String(),
		s.EnvId.String(),
		s.TaskId.String(),
		fmt.Sprintf("step%d", s.Index),
		runner.TaskStepLogName,
	)
}
