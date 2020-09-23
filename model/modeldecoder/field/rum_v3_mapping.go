// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package field

var rumV3Mapping = map[string]string{
	"abs_path":                    "ap",
	"action":                      "ac",
	"address":                     "ad",
	"agent":                       "a",
	"attributes":                  "at",
	"breakdown":                   "b",
	"cause":                       "ca",
	"classname":                   "cn",
	"code":                        "cd",
	"colno":                       "co",
	"connectEnd":                  "ce",
	"connectStart":                "cs",
	"context":                     "c",
	"context_line":                "cli",
	"culprit":                     "cl",
	"custom":                      "cu",
	"decoded_body_size":           "dbs",
	"destination":                 "dt",
	"domComplete":                 "dc",
	"domContentLoadedEventEnd":    "de",
	"domContentLoadedEventStart":  "ds",
	"domInteractive":              "di",
	"domLoading":                  "dl",
	"domainLookupEnd":             "le",
	"domainLookupStart":           "ls",
	"dropped":                     "dd",
	"duration":                    "d",
	"email":                       "em",
	"encoded_body_size":           "ebs",
	"env":                         "en",
	"environment":                 "en",
	"error":                       "e",
	"exception":                   "ex",
	"experience":                  "exp",
	"fetchStart":                  "fs",
	"filename":                    "f",
	"firstContentfulPaint":        "fp",
	"framework":                   "fw",
	"function":                    "fn",
	"handled":                     "hd",
	"headers":                     "he",
	"http":                        "h",
	"http_version":                "hve",
	"labels":                      "l",
	"language":                    "la",
	"largestContentfulPaint":      "lp",
	"level":                       "lv",
	"lineno":                      "li",
	"loadEventEnd":                "ee",
	"loadEventStart":              "es",
	"log":                         "log",
	"logger_name":                 "ln",
	"longtask":                    "lt",
	"marks":                       "k",
	"message":                     "mg",
	"metadata":                    "m",
	"method":                      "mt",
	"metricset":                   "me",
	"module":                      "mo",
	"name":                        "n",
	"navigationTiming":            "nt",
	"outcome":                     "o",
	"page":                        "p",
	"param_message":               "pmg",
	"parent_id":                   "pid",
	"parent_idx":                  "pi",
	"port":                        "po",
	"post_context":                "poc",
	"pre_context":                 "prc",
	"referer":                     "rf",
	"request":                     "q",
	"requestStart":                "qs",
	"resource":                    "rc",
	"result":                      "rt",
	"response":                    "r",
	"responseEnd":                 "re",
	"responseStart":               "rs",
	"runtime":                     "ru",
	"sampled":                     "sm",
	"samples":                     "sa",
	"sample_rate":                 "sr",
	"server-timing":               "set",
	"service":                     "se",
	"span":                        "y",
	"span.self_time.count":        "ysc",
	"span.self_time.sum.us":       "yss",
	"span_count":                  "yc",
	"stacktrace":                  "st",
	"start":                       "s",
	"started":                     "sd",
	"status_code":                 "sc",
	"subtype":                     "su",
	"sync":                        "sy",
	"tags":                        "g",
	"timeToFirstByte":             "fb",
	"trace_id":                    "tid",
	"transaction":                 "x",
	"transaction_id":              "xid",
	"transaction.breakdown.count": "xbc",
	"transaction.duration.count":  "xdc",
	"transaction.duration.sum.us": "xds",
	"transfer_size":               "ts",
	"type":                        "t",
	"url":                         "url",
	"user":                        "u",
	"username":                    "un",
	"value":                       "v",
	"version":                     "ve",
}

var rumV3InverseMapping = make(map[string]string)

func init() {
	for k, v := range rumV3Mapping {
		rumV3InverseMapping[v] = k
	}
}

// MapperFunc is the type of a function that maps from one field
// name to another.
type MapperFunc func(string) string

// Mapper returns a MapperFunc that maps from long to short names
// if shortFieldNames is true, and otherwise returns the identity
// function.
func Mapper(shortFieldNames bool) MapperFunc {
	if shortFieldNames {
		return rumV3Mapper
	}
	return identityMapper
}

// InverseMapper returns a MapperFunc that maps from short to long
// names if shortFieldNames is true, and otherwise returns the identity
// function.
func InverseMapper(shortFieldNames bool) MapperFunc {
	if shortFieldNames {
		return rumV3InverseMapper
	}
	return identityMapper
}

func rumV3Mapper(long string) string {
	if short, ok := rumV3Mapping[long]; ok {
		return short
	}
	return long
}

func rumV3InverseMapper(short string) string {
	if long, ok := rumV3InverseMapping[short]; ok {
		return long
	}
	return short
}

func identityMapper(s string) string {
	return s
}
