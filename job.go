package jobsd

import (
	"bytes"
	"database/sql/driver"
	"encoding/gob"
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm/schema"
)

// JobFunc .
type JobFunc struct {
	jobFunc reflect.Value
}

// check throws an error if the func is not valid and the args don't match func args
func (j *JobFunc) check(args []interface{}) error {
	if j.jobFunc.Kind() != reflect.Func {
		return errors.New("jobFunc is not a function")
	}

	theType := j.jobFunc.Type()
	// We expect 1 return value
	if theType.NumOut() != 1 {
		return errors.New("jobFunc needs to return one error")
	}

	// We expect the return value is an error
	errorInterface := reflect.TypeOf((*error)(nil)).Elem()
	if !theType.Out(0).Implements(errorInterface) {
		return errors.New("jobFunc return type needs to be an error")
	}

	// We expect the number of jobFunc args matches
	if theType.NumIn() != len(args) {
		return errors.New("the number of args do not match the jobs args")
	}

	// We expect the supplied args types are equal to the jobFuncs args
	for i := 0; i < theType.NumIn(); i++ {
		if reflect.ValueOf(args[i]).Kind() != theType.In(i).Kind() {
			return errors.New("the arg(s) types do not match job args types")
		}
	}

	return nil
}

// paramsCount returns the number of parameters required
func (j *JobFunc) paramsCount() int {
	return j.jobFunc.Type().NumIn()
}

// execute the JobFunc
func (j *JobFunc) execute(params []interface{}) error {
	if j.paramsCount() != len(params) {
		return errors.New("func parameters mismatch")
	}
	in := make([]reflect.Value, len(params))
	for k, param := range params {
		in[k] = reflect.ValueOf(param)
	}
	res := j.jobFunc.Call(in)

	if len(res) != 1 {
		return errors.New("func does not return a value")
	}

	if err, ok := res[0].Interface().(error); ok && err != nil {
		return err
	}
	return nil
}

// NewJobFunc .
func NewJobFunc(theFunc interface{}) *JobFunc {
	return &JobFunc{
		jobFunc: reflect.ValueOf(theFunc),
	}
}

// JobContainer .
type JobContainer struct {
	jobFunc             *JobFunc
	retryTimeout        time.Duration
	retryOnErrorLimit   int
	retryOnTimeoutLimit int
}

// RetryTimeout set the job default timeout
func (j *JobContainer) RetryTimeout(timeout time.Duration) *JobContainer {
	j.retryTimeout = timeout
	return j
}

// RetryErrorLimit set the job default number of retries on error
func (j *JobContainer) RetryErrorLimit(limit int) *JobContainer {
	j.retryOnErrorLimit = limit
	return j
}

// RetryTimeoutLimit set the job default number of retries on timeout
func (j *JobContainer) RetryTimeoutLimit(limit int) *JobContainer {
	j.retryOnTimeoutLimit = limit
	return j
}

// Args holds job func parameters used to run a job. It can be serialized for DB storage
type Args []interface{}

// GormDataType .
func (p Args) GormDataType() string {
	return string(schema.String)
}

// Scan scan value into []
func (p *Args) Scan(value interface{}) error {
	data, ok := value.(string)
	if !ok {
		return errors.New(fmt.Sprint("failed to unmarshal params value:", value))
	}
	r := bytes.NewReader([]byte(data))
	dec := gob.NewDecoder(r)
	return dec.Decode(p)
}

// Value return params value, implement driver.Valuer interface
func (p Args) Value() (driver.Value, error) {
	if len(p) == 0 {
		return nil, nil
	}
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	enc.Encode(p)
	return string(buf.Bytes()), nil
}
