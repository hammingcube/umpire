package main

import (
	"bytes"
	"encoding/json"
	"github.com/maddyonline/umpire"
	"github.com/maddyonline/umpire/pkg/dockerutils"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

var raw = `
{
	"problem": {"id":"two-sum"},
	"files": [
		{
			"Name": "main.py",
			"Content": "def getInput():\r\n    return [[[2, 7, 11, 15], 9], [[2, 11, 7, 15], 9]]\r\nclass Solution(object):\r\n    def twoSum(self, nums, target):\r\n        \"\"\"\r\n        :type nums: List[int]\r\n        :type target: int\r\n        :rtype: List[int]\r\n        \"\"\"\r\n        h = dict()\r\n        for i, e in enumerate(nums):\r\n            if target - e in h:\r\n                return [h[target-e], i]\r\n            h[e] = i\r\n\r\nsoln = Solution()\r\nfor nums, target in getInput():\r\n    indices = soln.twoSum(nums, target)\r\n    print('{} {}'.format(indices[0], indices[1]))\r\n\r\n"
		}
	],
	"language": "python",
	"stdin": ""
}
`

func init() {
}

func jsonPostRequest(t *testing.T, path string, body []byte) *http.Request {
	req, err := http.NewRequest("POST", path, bytes.NewReader(body))
	if err != nil {
		t.Error(err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}

func assertMapEqual(t *testing.T, got, expected map[string]string) {
	for k1, v1 := range expected {
		v2, ok := got[k1]
		if !ok {
			t.Errorf("key %s missing", k1)
		}
		if v2 != v1 {
			t.Errorf("key %q: got %q, expected: %q", k1, v1, v2)
		}
	}
}

func TestEndToEnd(t *testing.T) {
	agent := &umpire.Agent{
		Client: dockerutils.NewClient(),
	}
	if agent.Client == nil {
		t.Fatalf("Failed to initialize docker client")
	}
	server := NewUmpireServer(agent)
	e := server.e
	e.Logger.SetOutput(ioutil.Discard)
	req := jsonPostRequest(t, "/execute", []byte(raw))
	rw := httptest.NewRecorder()
	e.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("StatusCode: should receive %d instead got %d", http.StatusOK, rw.Code)
	}
	got := map[string]string{}
	json.NewDecoder(rw.Body).Decode(&got)
	expected := map[string]string{"stderr": "", "status": "pass", "details": "", "stdout": "0 1\n0 2\n"}
	assertMapEqual(t, got, expected)
}
