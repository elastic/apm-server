// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package profiling

import (
	"fmt"

	"github.com/elastic/apm-server/x-pack/apm-server/profiling/libpf"
)

// CollectTracesAndCounts reads the RPC request and builds a list of traces and their counts
// timestamps / counts using the libpf type.
func CollectTracesAndCounts(in *AddCountsForTracesRequest) ([]libpf.TraceAndCounts, error) {
	timestamps := in.GetTimestamps()
	hiTraceHashes := in.GetHiTraceHashes()
	loTraceHashes := in.GetLoTraceHashes()
	counts := in.GetCounts()
	commsIdx := in.GetCommsIdx()

	numTimestamps := len(timestamps)
	numHiHashs := len(hiTraceHashes)
	numLoHashs := len(loTraceHashes)
	numCounts := len(counts)
	numComms := len(commsIdx)

	podNamesIdx := in.GetPodNamesIdx()
	containerNamesIdx := in.GetContainerNamesIdx()
	stringTable := in.GetStringTable()
	numStrings := uint32(len(stringTable))

	// Sanity checks. Should never fail unless the HA is broken.
	if numCounts != numHiHashs || numCounts != numLoHashs {
		return nil, fmt.Errorf("mismatch in number of first hash components (%d) "+
			"and second hash components (%d) to counts (%d)",
			numHiHashs, numLoHashs, numCounts)
	}

	if numCounts != numTimestamps {
		return nil, fmt.Errorf("mismatch in number of counts (%d) and timestamps (%d)",
			numCounts, numTimestamps)
	}

	if numCounts != numComms {
		return nil, fmt.Errorf("mismatch in number of counts (%d) and comms (%d)",
			numCounts, numComms)
	}

	numTraces := uint32(numCounts)
	for i := uint32(0); i < numTraces; i++ {
		if commIdx := commsIdx[i]; commIdx >= numStrings {
			return nil, fmt.Errorf("commsIdx[%d] %d exceeds stringTable length: %d, "+
				"skipped %d traces",
				i, commIdx, numStrings, numTraces)
		}

		if podNameIdx, ok := podNamesIdx[i]; ok && podNameIdx >= numStrings {
			return nil, fmt.Errorf("podNamesIdx[%d] %d exceeds stringTable length: %d, "+
				"skipped %d traces",
				i, podNameIdx, numStrings, numTraces)
		}

		if containerNameIdx, ok := containerNamesIdx[i]; ok && containerNameIdx >= numStrings {
			return nil, fmt.Errorf("containerNamesIdx[%d] %d exceeds stringTable length: %d, "+
				"skipped %d traces",
				i, containerNameIdx, numStrings, numTraces)
		}
	} // End sanity checks

	traces := make([]libpf.TraceAndCounts, numTraces)
	for i := uint32(0); i < numTraces; i++ {
		traceHash := libpf.NewTraceHash(hiTraceHashes[i], loTraceHashes[i])
		traces[i].Timestamp = libpf.UnixTime32(timestamps[i])
		traces[i].Hash = traceHash
		traces[i].Count = uint16(counts[i])
		traces[i].Comm = stringTable[commsIdx[i]]

		if podNameIdx, ok := podNamesIdx[i]; ok {
			traces[i].PodName = stringTable[podNameIdx]
		}
		if containerNameIdx, ok := containerNamesIdx[i]; ok {
			traces[i].ContainerName = stringTable[containerNameIdx]
		}
	}
	return traces, nil
}

// CollectTracesAndFrames reads the RPC request and builds a list of traces and their frames
// using the libpf type.
func CollectTracesAndFrames(in *SetFramesForTracesRequest) ([]*libpf.Trace, error) {
	hiTraceHashes := in.GetHiTraceHashes()
	loTraceHashes := in.GetLoTraceHashes()
	frameCounts := in.GetFrameCounts()
	numHiHashs := len(hiTraceHashes)
	numLoHashs := len(loTraceHashes)
	numFrameCounts := len(frameCounts)

	types := in.GetTypes()
	hiContainers := in.GetHiContainers()
	loContainers := in.GetLoContainers()
	offsets := in.GetOffsets()
	numTypes := len(types)
	numHiContainers := len(hiContainers)
	numLoContainers := len(loContainers)
	numOffsets := len(offsets)

	// Sanity checks. Should never fail unless the HA is broken.
	if numTypes != numHiContainers || numTypes != numLoContainers || numTypes != numOffsets {
		return nil, fmt.Errorf("mismatch in number of types (%d) hiContainers (%d) "+
			"loContainers (%d) offsets (%d)",
			numTypes, numHiContainers, numLoContainers, numOffsets)
	}

	totalFrameCount := 0
	for _, count := range frameCounts {
		totalFrameCount += int(count)
	}
	if totalFrameCount != numTypes {
		return nil, fmt.Errorf("mismatch in total frame count (%d) and number of types (%d)",
			totalFrameCount, numTypes)
	}

	if numFrameCounts != numHiHashs || numFrameCounts != numLoHashs {
		return nil, fmt.Errorf("mismatch in number of first hash components (%d) "+
			"and second hash components (%d) to frame counts (%d)",
			numHiHashs, numLoHashs, numFrameCounts)
	}
	// End sanity checks

	numTraces := uint32(numFrameCounts)
	traces := make([]*libpf.Trace, numTraces)
	// Keeps track of current position in flattened arrays
	j := 0
	for i := uint32(0); i < numTraces; i++ {
		numFrames := int(frameCounts[i])
		trace := &libpf.Trace{
			Hash:       libpf.NewTraceHash(hiTraceHashes[i], loTraceHashes[i]),
			Files:      make([]libpf.FileID, numFrames),
			Linenos:    make([]libpf.AddressOrLineno, numFrames),
			FrameTypes: make([]libpf.FrameType, numFrames),
		}

		for k := 0; k < numFrames; k++ {
			trace.Files[k] = libpf.NewFileID(hiContainers[j], loContainers[j])
			trace.Linenos[k] = libpf.AddressOrLineno(offsets[j])
			trace.FrameTypes[k] = libpf.FrameType(types[j])
			j++
		}

		traces[i] = trace
		//log.Debugf("Received details for trace with hash: 0x%x", trace.Hash)
	}

	return traces, nil
}

// CollectFrameMetadata reads the RPC request and builds a list of frame metadata using
// the libpf type.
func CollectFrameMetadata(in *AddFrameMetadataRequest) ([]*libpf.FrameMetadata, error) {
	hiFileIDs := in.GetHiFileIDs()
	loFileIDs := in.GetLoFileIDs()
	addressOrLines := in.GetAddressOrLines()
	hiSourceIDs := in.GetHiSourceIDs()
	loSourceIDs := in.GetLoSourceIDs()
	lineNumbers := in.GetLineNumbers()
	functionNamesIdx := in.GetFunctionNamesIdx()
	functionOffsets := in.GetFunctionOffsets()
	filenamesIdx := in.GetFilenamesIdx()

	numHiFileIDs := len(hiFileIDs)
	numLoFileIDs := len(loFileIDs)
	numAddressOrLines := len(addressOrLines)
	numHiSourceIDs := len(hiSourceIDs)
	numLoSourceIDs := len(loSourceIDs)
	numLineNumbers := len(lineNumbers)
	numFunctionNames := len(functionNamesIdx)
	numFunctionOffsets := len(functionOffsets)
	numFilenames := len(filenamesIdx)

	// Sanity checks. Should never fail unless the HA is broken or client is malicious.
	if numHiFileIDs != numLoFileIDs || numHiFileIDs != numAddressOrLines ||
		numHiFileIDs != numHiSourceIDs || numHiFileIDs != numLoSourceIDs ||
		numHiFileIDs != numLineNumbers || numHiFileIDs != numFunctionNames ||
		numHiFileIDs != numFunctionOffsets || numHiFileIDs != numFilenames {
		return nil, fmt.Errorf("mismatch in number of HiFileIDs (%d) LoFileIDs (%d) "+
			"AddressOrLines (%d) HiSourceIDs (%d) LoSourceIDs (%d) LineNumbers (%d) "+
			"FunctionNames (%d) FunctionOffsets (%d) Filenames (%d)",
			numHiFileIDs, numLoFileIDs, numAddressOrLines, numHiSourceIDs, numLoSourceIDs,
			numLineNumbers, numFunctionNames, numFunctionOffsets, numFilenames)
	}
	// End sanity checks

	stringTable := in.GetStringTable()
	numStrings := uint32(len(stringTable))
	frames := make([]*libpf.FrameMetadata, numHiFileIDs)

	for i := 0; i < numHiFileIDs; i++ {
		functionNameIdx := functionNamesIdx[i]
		if functionNameIdx >= numStrings {
			return nil, fmt.Errorf("functionNamesIdx[%d] %d exceeds stringTable length: %d",
				i, functionNameIdx, numStrings)
		}

		filenameIdx := filenamesIdx[i]
		if filenameIdx >= numStrings {
			return nil, fmt.Errorf("filenamesIdx[%d] %d exceeds stringTable length: %d",
				i, filenameIdx, numStrings)
		}

		frames[i] = &libpf.FrameMetadata{
			FileID:         libpf.NewFileID(hiFileIDs[i], loFileIDs[i]),
			SourceID:       libpf.NewFileID(hiSourceIDs[i], loSourceIDs[i]),
			AddressOrLine:  libpf.AddressOrLineno(addressOrLines[i]),
			LineNumber:     libpf.SourceLineno(lineNumbers[i]),
			FunctionOffset: functionOffsets[i],
			FunctionName:   stringTable[functionNameIdx],
			Filename:       stringTable[filenameIdx],
		}
	}
	return frames, nil
}
