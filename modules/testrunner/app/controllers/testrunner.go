package controllers

import (
	"bytes"
	"fmt"
	"github.com/robfig/revel"
	"github.com/robfig/revel/modules/testrunner/app"
	"html"
	"html/template"
	"reflect"
	"strings"
)

type TestRunner struct {
	*rev.Controller
}

type TestSuiteDesc struct {
	Name  string
	Tests []TestDesc
}

type TestDesc struct {
	Name string
}

type TestSuiteResult struct {
	Name    string
	Passed  bool
	Results []TestResult
}

type TestResult struct {
	Name      string
	Passed    bool
	ErrorHtml template.HTML
}

var NONE = []reflect.Value{}

func (c TestRunner) Index() rev.Result {
	var testSuites []TestSuiteDesc
	for _, testSuite := range rev.TestSuites {
		testSuites = append(testSuites, DescribeSuite(testSuite))
	}
	return c.Render(testSuites)
}

// Run runs a single test, given by the argument. 
func (c TestRunner) Run(suite, test string) rev.Result {
	result := TestResult{Name: test}
	for _, testSuite := range rev.TestSuites {
		t := reflect.TypeOf(testSuite).Elem()
		if t.Name() != suite {
			continue
		}

		// Found the suite, create a new instance and run the named method.
		v := reflect.New(t)
		func() {
			defer func() {
				if err := recover(); err != nil {
					error := rev.NewErrorFromPanic(err)
					if error == nil {
						result.ErrorHtml = template.HTML(html.EscapeString(fmt.Sprint(err)))
					} else {
						var buffer bytes.Buffer
						tmpl, _ := rev.MainTemplateLoader.Template("TestRunner/FailureDetail.html")
						tmpl.Render(&buffer, error)
						result.ErrorHtml = template.HTML(buffer.String())
					}
				}
			}()

			// Initialize the test suite with a NewTestSuite()
			testSuiteInstance := v.Elem().FieldByName("TestSuite")
			testSuiteInstance.Set(reflect.ValueOf(rev.NewTestSuite()))

			// Call Before(), call the test, and call After().
			if m := v.MethodByName("Before"); m.IsValid() {
				m.Call(NONE)
			}
			v.MethodByName(test).Call(NONE)
			if m := v.MethodByName("After"); m.IsValid() {
				m.Call(NONE)
			}

			// No panic means success.
			result.Passed = true
		}()
		break
	}
	return c.RenderJson(result)
}

// List returns a JSON list of test suites and tests.
// Used by the "test" command line tool.
func (c TestRunner) List() rev.Result {
	var testSuites []TestSuiteDesc
	for _, testSuite := range rev.TestSuites {
		testSuites = append(testSuites, DescribeSuite(testSuite))
	}
	return c.RenderJson(testSuites)
}

func DescribeSuite(testSuite interface{}) TestSuiteDesc {
	t := reflect.TypeOf(testSuite).Elem()

	// Get a list of methods of the embedded test type.
	super := t.Field(0).Type
	superMethodNameSet := map[string]struct{}{}
	for i := 0; i < super.NumMethod(); i++ {
		superMethodNameSet[super.Method(i).Name] = struct{}{}
	}

	// Get a list of methods on the test suite that take no parameters, return
	// no results, and were not part of the embedded type's method set.
	var tests []TestDesc
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		mt := m.Type
		_, isSuperMethod := superMethodNameSet[m.Name]
		if mt.NumIn() == 1 &&
			mt.NumOut() == 0 &&
			mt.In(0) == t &&
			!isSuperMethod &&
			strings.HasPrefix(m.Name, "Test") {
			tests = append(tests, TestDesc{m.Name})
		}
	}

	return TestSuiteDesc{
		Name:  t.Name(),
		Tests: tests,
	}
}

func init() {
	rev.RegisterPlugin(app.TestRunnerPlugin{})
}