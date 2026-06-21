package main

import (
	"encoding/json"
	"log"

	"github.com/google/uuid"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type BroadcastRequest struct {
	maelstrom.MessageBody
	Message int `json:"message"`
}

type Topology map[string][]string

func (t Topology) NeighborNodes(id string) []string {
	if t == nil {
		return nil
	}

	return t[id]
}

type TopologyRequest struct {
	maelstrom.MessageBody
	Topology Topology `json:"topology"`
}

type Store struct {
	messages map[int]struct{}
	topology Topology
}

func (s *Store) WriteMessage(msg int) bool {
	if _, ok := s.messages[msg]; ok {
		return false
	}
	s.messages[msg] = struct{}{}
	return true
}

func (s *Store) ReadMessages() []int {
	var messages []int
	for msg := range s.messages {
		messages = append(messages, msg)
	}

	return messages
}

func main() {
	n := maelstrom.NewNode()
	store := &Store{
		messages: make(map[int]struct{}),
	}

	n.Handle("echo", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		body["type"] = "echo_ok"
		return n.Reply(msg, body)
	})

	n.Handle("generate", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		id, err := uuid.NewV7()
		if err != nil {
			return err
		}

		body["type"] = "generate_ok"
		body["id"] = id.String()

		return n.Reply(msg, body)
	})

	n.Handle("broadcast", func(msg maelstrom.Message) error {
		var body BroadcastRequest
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		// Write message to neighbours only if the message was successfully written to the current node.
		// We assume that if a node has a message, it's neighbours have the message too.
		if ok := store.WriteMessage(body.Message); ok {
			nodes := store.topology.NeighborNodes(n.ID())
			for _, node := range nodes {
				if err := n.Send(node, msg.Body); err != nil {
					log.Printf("Error sending message to %s: %s", node, err)
				}
			}
		}

		return n.Reply(msg, map[string]string{
			"type": "broadcast_ok",
		})
	})

	n.Handle("read", func(msg maelstrom.Message) error {
		return n.Reply(msg, map[string]any{
			"type":     "read_ok",
			"messages": store.ReadMessages(),
		})
	})

	n.Handle("topology", func(msg maelstrom.Message) error {
		var body TopologyRequest
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}
		store.topology = body.Topology
		return n.Reply(msg, map[string]string{
			"type": "topology_ok",
		})
	})

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}
