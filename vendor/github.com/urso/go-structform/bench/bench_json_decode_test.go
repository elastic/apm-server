package bench

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	stdjson "encoding/json"
)

func BenchmarkDecodeBeatsEvents(b *testing.B) {
	runPaths := func(paths ...string) func(*testing.B) {
		return func(b *testing.B) {
			jsonContent := readFile(paths...)

			b.Run("std-json",
				makeBenchmarkDecodeBeatsEvents(stdJSONBufDecoder, jsonContent))

			// panic: fails to parse events
			//b.Run("jsoniter",
			//	makeBenchmarkDecodeBeatsEvents(jsoniterBufDecoder, jsonContent))

			b.Run("structform-json",
				makeBenchmarkDecodeBeatsEvents(structformJSONBufDecoder(0), jsonContent))
			b.Run("structform-json-keycache",
				makeBenchmarkDecodeBeatsEvents(structformJSONBufDecoder(1000), jsonContent))

			ubjsonContent := readFileEncoded(structformUBJSONEncoder, paths...)
			b.Run("structform-ubjson",
				makeBenchmarkDecodeBeatsEvents(structformUBJSONBufDecoder(0), ubjsonContent))
			b.Run("structform-ubjson-keycache",
				makeBenchmarkDecodeBeatsEvents(structformUBJSONBufDecoder(1000), ubjsonContent))

			cborContent := readFileEncoded(structformCBORLEncoder, paths...)
			b.Run("structform-cborl",
				makeBenchmarkDecodeBeatsEvents(structformCBORLBufDecoder(0), cborContent))
			b.Run("structform-cborl-keycache",
				makeBenchmarkDecodeBeatsEvents(structformCBORLBufDecoder(1000), cborContent))

		}
	}

	b.Run("packetbeat", runPaths("files/packetbeat_events.json"))
	b.Run("metricbeat", runPaths("files/metricbeat_events.json"))
	b.Run("filebeat", runPaths("files/filebeat_events.json"))
}

func BenchmarkEncodeBeatsEvents(b *testing.B) {
	runPaths := func(paths ...string) func(*testing.B) {
		events := loadEvents(paths...)
		return func(b *testing.B) {
			b.Run("std-json", makeBenchmarkEncodeEvents(stdJSONEncoder, events))
			b.Run("structform-json", makeBenchmarkEncodeEvents(structformJSONEncoder, events))
			b.Run("structform-ubjson", makeBenchmarkEncodeEvents(structformUBJSONEncoder, events))
			b.Run("structform-cborl", makeBenchmarkEncodeEvents(structformCBORLEncoder, events))
		}
	}

	b.Run("packetbeat", runPaths("files/packetbeat_events.json"))
	b.Run("metricbeat", runPaths("files/metricbeat_events.json"))
	b.Run("filebeat", runPaths("files/filebeat_events.json"))
}

func BenchmarkTranscodeBeatsEvents(b *testing.B) {
	runPaths := func(paths ...string) func(*testing.B) {
		return func(b *testing.B) {
			b.Run("structform-cborl->json", makeBenchmarkTranscodeEvents(
				structformCBORLEncoder,
				makeCBORL2JSONTranscoder,
				paths...,
			))
			b.Run("structform-ubjson->json", makeBenchmarkTranscodeEvents(
				structformUBJSONEncoder,
				makeUBJSON2JSONTranscoder,
				paths...,
			))
		}
	}

	b.Run("packetbeat", runPaths("files/packetbeat_events.json"))
	b.Run("metricbeat", runPaths("files/metricbeat_events.json"))
	b.Run("filebeat", runPaths("files/filebeat_events.json"))
}

func makeBenchmarkDecodeBeatsEvents(
	factory decoderFactory,
	content []byte,
) func(*testing.B) {

	return func(b *testing.B) {
		b.SetBytes(int64(len(content)))
		for i := 0; i < b.N; i++ {
			decode := factory(content)

			for {
				var to map[string]interface{}

				if err := decode(&to); err != nil {
					if err != io.EOF {
						b.Error(err)
					}
					break
				}
			}
		}
	}
}

func makeBenchmarkEncodeEvents(factory encoderFactory, events []map[string]interface{}) func(*testing.B) {
	var buf bytes.Buffer
	buf.Grow(16 * 1024)
	encode := factory(&buf)

	return func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var written int64

			for _, event := range events {
				buf.Reset()
				if err := encode(event); err != nil {
					b.Error(err)
					return
				}
				written += int64(buf.Len())
			}
			b.SetBytes(written)
		}
	}
}

func makeBenchmarkTranscodeEvents(
	fEnc encoderFactory,
	fTransc transcodeFactory,
	paths ...string,
) func(b *testing.B) {
	content := readFileEncoded(fEnc, paths...)

	var buf bytes.Buffer
	transcode := fTransc(&buf)
	return func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buf.Reset()
			err := transcode(content)
			if err != nil {
				b.Error(err)
				return
			}

			content := buf.Bytes()
			n := len(content)
			b.SetBytes(int64(n))
		}
	}
}

func loadEvents(paths ...string) []map[string]interface{} {
	content := readFile(paths...)

	var events []map[string]interface{}
	dec := stdjson.NewDecoder(bytes.NewReader(content))
	for {
		var e map[string]interface{}
		if err := dec.Decode(&e); err != nil {
			if err == io.EOF {
				break
			}

			panic(err)
		}

		events = append(events, e)
	}

	return events
}

func readFileEncoded(encFactory encoderFactory, paths ...string) []byte {
	var buf bytes.Buffer
	enc := encFactory(&buf)

	events := loadEvents(paths...)
	for _, event := range events {
		err := enc(event)
		if err != nil {
			panic(err)
		}
	}

	return buf.Bytes()
}

func readFile(paths ...string) []byte {
	var buf bytes.Buffer

	for _, p := range paths {
		content, err := ioutil.ReadFile(p)
		if err != nil {
			if err != nil {
				panic(err)
			}
		}

		buf.Write(content)
	}

	return buf.Bytes()
}
