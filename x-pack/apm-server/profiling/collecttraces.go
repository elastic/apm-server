// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package profiling

import (
	"fmt"

	"github.com/elastic/apm-server/x-pack/apm-server/profiling/libpf"
)

// CollectTracesAndCounts reads the RPC request and builds a list of traces and their counts
// using the libpf type. We export this function to allow other collectors to use the same logic.
func CollectTracesAndCounts(in *AddCountsForTracesRequest) ([]libpf.TraceAndCounts, error) {
	hiTraceHashes := in.GetHiTraceHashes()
	loTraceHashes := in.GetLoTraceHashes()
	counts := in.GetCounts()
	commsIdx := in.GetCommsIdx()

	numHiHashs := len(hiTraceHashes)
	numLoHashs := len(loTraceHashes)
	numCounts := len(counts)
	numComms := len(commsIdx)

	podNamesIdx := in.GetPodNamesIdx()
	containerNamesIdx := in.GetContainerNamesIdx()
	uniqMetadata := in.GetUniqueMetadata()
	numMetadata := uint32(len(uniqMetadata))

	// Sanity checks. Should never fail unless the HA is broken.
	if numCounts != numHiHashs || numCounts != numLoHashs {
		return nil, fmt.Errorf("mismatch in number of first hash components (%d) "+
			"and second hash components (%d) to counts (%d)",
			numHiHashs, numLoHashs, numCounts)
	}

	if numCounts != numComms {
		return nil, fmt.Errorf("mismatch in number of counts (%d) and comms (%d)",
			numCounts, numComms)
	}

	numTraces := uint32(numCounts)
	for i := uint32(0); i < numTraces; i++ {
		if commIdx := commsIdx[i]; commIdx >= numMetadata {
			return nil, fmt.Errorf("commsIdx[%d] %d exceeds metadata length: %d, "+
				"skipped %d traces",
				i, commIdx, numMetadata, numTraces)
		}

		if podNameIdx, ok := podNamesIdx[i]; ok && podNameIdx >= numMetadata {
			return nil, fmt.Errorf("podNamesIdx[%d] %d exceeds metadata length: %d, "+
				"skipped %d traces",
				i, podNameIdx, numMetadata, numTraces)
		}

		if containerNameIdx, ok := containerNamesIdx[i]; ok && containerNameIdx >= numMetadata {
			return nil, fmt.Errorf("containerNamesIdx[%d] %d exceeds metadata length: %d, "+
				"skipped %d traces",
				i, containerNameIdx, numMetadata, numTraces)
		}
	} // End sanity checks

	traces := make([]libpf.TraceAndCounts, numTraces)
	for i := uint32(0); i < numTraces; i++ {
		traceHash := libpf.NewTraceHash(hiTraceHashes[i], loTraceHashes[i])
		traces[i].Hash = traceHash
		traces[i].Count = uint16(counts[i])
		traces[i].Comm = uniqMetadata[commsIdx[i]]

		if podNameIdx, ok := podNamesIdx[i]; ok {
			traces[i].PodName = uniqMetadata[podNameIdx]
		}
		if containerNameIdx, ok := containerNamesIdx[i]; ok {
			traces[i].ContainerName = uniqMetadata[containerNameIdx]
		}
	}
	return traces, nil
}

// CollectTracesAndFrames reads the RPC request and builds a list of traces and their frames
// using the libpf type. We export this function to allow other collectors to use the same logic.
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

	commsIdx := in.GetCommsIdx()
	podNamesIdx := in.GetPodNamesIdx()
	containerNamesIdx := in.GetContainerNamesIdx()
	uniqMetadata := in.GetUniqueMetadata()
	numComms := len(commsIdx)
	numMetadata := uint32(len(uniqMetadata))

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

	if numFrameCounts != numComms {
		return nil, fmt.Errorf("mismatch in number of frame counts (%d) and comms (%d)",
			numFrameCounts, numComms)
	}

	numTraces := uint32(numFrameCounts)
	for i := uint32(0); i < numTraces; i++ {
		if commIdx := commsIdx[i]; commIdx >= numMetadata {
			return nil, fmt.Errorf("commsIdx[%d] %d exceeds metadata length: %d, "+
				"skipped %d traces",
				i, commIdx, numMetadata, numTraces)
		}

		if podNameIdx, ok := podNamesIdx[i]; ok && podNameIdx >= numMetadata {
			return nil, fmt.Errorf("podNamesIdx[%d] %d exceeds metadata length: %d, "+
				"skipped %d traces",
				i, podNameIdx, numMetadata, numTraces)
		}

		if containerNameIdx, ok := containerNamesIdx[i]; ok && containerNameIdx >= numMetadata {
			return nil, fmt.Errorf("containerNamesIdx[%d] %d exceeds metadata length: %d, "+
				"skipped %d traces",
				i, containerNameIdx, numMetadata, numTraces)
		}
	} // End sanity checks

	traces := make([]*libpf.Trace, numTraces)
	// Keeps track of current position in flattened arrays
	j := 0
	for i := uint32(0); i < numTraces; i++ {
		numFrames := int(frameCounts[i])
		trace := &libpf.Trace{
			Hash:       libpf.NewTraceHash(hiTraceHashes[i], loTraceHashes[i]),
			Files:      make([]libpf.FileID, numFrames),
			Linenos:    make([]libpf.AddressOrLineno, numFrames),
			FrameTypes: make([]libpf.InterpType, numFrames),
		}

		for k := 0; k < numFrames; k++ {
			trace.Files[k] = libpf.NewFileID(hiContainers[j], loContainers[j])
			trace.Linenos[k] = libpf.AddressOrLineno(offsets[j])
			trace.FrameTypes[k] = libpf.InterpType(types[j])
			j++
		}

		trace.Comm = uniqMetadata[commsIdx[i]]
		if podNameIdx, ok := podNamesIdx[i]; ok {
			trace.PodName = uniqMetadata[podNameIdx]
		}

		if containerNameIdx, ok := containerNamesIdx[i]; ok {
			trace.ContainerName = uniqMetadata[containerNameIdx]
		}

		traces[i] = trace
		//log.Debugf("Received details for trace with hash: 0x%x", trace.Hash)
	}

	return traces, nil
}
