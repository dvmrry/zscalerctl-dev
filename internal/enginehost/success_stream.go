package enginehost

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"

	"github.com/dvmrry/zscalerctl/internal/enginewire"
)

type successCursor struct {
	plan *successPlan

	itemIndex  int
	phase      int
	chunkIndex int
	hasher     hash.Hash
	completed  bool
}

func newSuccessCursor(plan *successPlan) *successCursor {
	return &successCursor{plan: plan}
}

func (c *successCursor) Next(
	id enginewire.SafeInteger,
	sequence enginewire.SafeInteger,
) (enginewire.ServerFrame, bool, error) {
	if c == nil || c.plan == nil || c.completed {
		return nil, false, fmt.Errorf("success stream is exhausted")
	}
	if c.itemIndex >= len(c.plan.items) {
		c.completed = true
		return enginewire.Completed[enginewire.CompletionResult]{
			Type: "completed", ID: id, Sequence: sequence, Result: c.plan.result,
		}, true, nil
	}

	item := c.plan.items[c.itemIndex]
	if !item.fragmented {
		c.itemIndex++
		return enginewire.Item[enginewire.ItemValue]{
			Type: "item", ID: id, Sequence: sequence, Kind: item.kind, Item: item.value,
		}, false, nil
	}
	itemID, err := safeWireCount(c.itemIndex + 1)
	if err != nil {
		return nil, false, err
	}

	switch c.phase {
	case 0:
		payloadBytes, err := safeWireCount(len(item.payload))
		if err != nil {
			return nil, false, err
		}
		c.phase = 1
		c.chunkIndex = 0
		c.hasher = sha256.New()
		return enginewire.ItemBegin{
			Type: "item_begin", ID: id, Sequence: sequence, ItemID: itemID,
			Kind: item.kind, Encoding: "json", Bytes: payloadBytes,
		}, false, nil
	case 1:
		if c.chunkIndex < item.chunks {
			start := c.chunkIndex * enginewire.FragmentChunkBytes
			end := min(start+enginewire.FragmentChunkBytes, len(item.payload))
			chunkBytes := item.payload[start:end]
			if _, err := c.hasher.Write(chunkBytes); err != nil {
				return nil, false, fmt.Errorf("hash item chunk: %w", err)
			}
			chunkIndex, err := safeWireCount(c.chunkIndex)
			if err != nil {
				return nil, false, err
			}
			frame := enginewire.ItemChunk{
				Type: "item_chunk", ID: id, Sequence: sequence, ItemID: itemID,
				Index: chunkIndex,
				Data:  base64.StdEncoding.EncodeToString(chunkBytes),
			}
			c.chunkIndex++
			return frame, false, nil
		}
		if got := hex.EncodeToString(c.hasher.Sum(nil)); got != item.digest {
			return nil, false, fmt.Errorf("fragment digest changed after success commit")
		}
		c.phase = 0
		c.itemIndex++
		chunkCount, err := safeWireCount(item.chunks)
		if err != nil {
			return nil, false, err
		}
		return enginewire.ItemEnd{
			Type: "item_end", ID: id, Sequence: sequence, ItemID: itemID,
			Chunks: chunkCount, SHA256: item.digest,
		}, false, nil
	default:
		return nil, false, fmt.Errorf("invalid success stream phase")
	}
}
