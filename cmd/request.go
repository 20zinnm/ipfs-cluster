package cmd

import (
	"time"
	"fmt"
	"strings"
	"github.com/ipfs/ipfs-cluster/logger"
	"github.com/Sirupsen/logrus"
	"net/http"
	"golang.org/x/net/context"
)

func request(method, path string, body io.Reader, args ...string) *http.Response {
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(timeout)*time.Second)
	defer cancel()

	u := defaultProtocol + "://" + host + path
	// turn /a/{param0}/{param1} into /a/this/that
	for i, a := range args {
		p := fmt.Sprintf("{param%d}", i)
		u = strings.Replace(u, p, a, 1)
	}
	u = strings.TrimSuffix(u, "/")

	logrus.WithFields(logrus.Fields{
		"method": method,
		"url":    u,
	}).Debug("request")

	r, err := http.NewRequest(method, u, body)
	if err != nil {
		logrus.WithError(err).Error("error creating request")
		return nil
	}
	logrus.Info("creating request")
	r.WithContext(ctx)

	client := &http.Client{}
	resp, err := client.Do(r)
	if err != nil {
		logrus.WithError(err).Error("error making request")
	}

	logrus.WithField("host", host).Info("performing request")

	return resp
}
