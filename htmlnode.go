/*
   Copyright 2015 The Htmlnode Authors. See the AUTHORS file at the
   top-level directory of this distribution and at
   <https://xi2.org/x/htmlnode/m/AUTHORS>.

   This file is part of Htmlnode.

   Htmlnode is free software: you can redistribute it and/or modify it
   under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   Htmlnode is distributed in the hope that it will be useful, but
   WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
   General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with Htmlnode.  If not, see <https://www.gnu.org/licenses/>.
*/

// Package htmlnode provides functions for searching, traversing and
// printing parsed HTML. It is based on the html.Node data type from
// the golang.org/x/net/html package.
//
// Note: The API is presently experimental and may change.
//
// Example
//
// The most useful function in the package is probably Find, and is
// best illustrated with an example.
//
// Suppose we have a parsed HTML tree as follows:
//
//       R
//         E html
//           E head
//           E body
//             E div id="lowframe" style="position: fixed;" ...
//             C  #lowframe
//             E div id="topbar"
//               E div class="container"
//                 E div class="top-heading" id="heading-wide"
//                   E a href="/"
//                     T The Go Programming Language
//                 E div class="top-heading" id="heading-narrow"
//                   E a href="/"
//   (1)               T Go
//                 E a href="#" id="menu-button"
//                   E span id="menu-button-arrow"
//                     T ▽
//                 E form method="GET" action="/search"
//                   E div id="menu"
//   (2)               E a href="/doc/"
//                       T Documents
//   (3)               E a href="/pkg/"
//                       T Packages
//   (4)               E a href="/project/"
//                       T The Project
//   (5)               E a href="/help/"
//                       T Help
//   (6)               E a href="/blog/"
//                       T Blog
//   (7)               E a href="http://play.golang.org/" ...
//                       T Play
//                     E input type="text" id="search" name="q" ...
//
// This is actually a section of the golang.org front page and
// demonstrates the Print function in this package. Some of the nodes
// are numbered and referred to below.
//
// The following call to Find will return the nodes numbered (2) ->
// (7) in a slice:
//
//   Find(root, `<form><div><a>`)
//
// This is because tracing these nodes down from the root, they end
// with the three element nodes <form>, <div> and <a>. It does not
// matter that the <div> specified is missing the id="menu"
// attribute. All that matters are that its attributes (none) are a
// subset of those in the tree. If, however, we were to use:
//
//   Find(root, `<form><div id="someotherid"><a>`)
//
// we would get no results since the id="someotherid" does not match
// in the tree.
//
// Another example. Calling:
//
//   Find(root, `<a href=/>Go`)
//
// returns node (1), so you can pick out non-element nodes too.
//
// A note on fragments
//
// The fragment passed to Find has to parse in the context of having a
// generic element node as its parent. So it is fine to call:
//
//   Find(root, `<table><tr><td>`)
//
// on some document, but if you were to instead use:
//
//   Find(root, `<tr><td>`)
//
// you would get an empty slice, since the fragment `<tr><td>` is not
// valid directly under an arbitrary element node and will not
// parse. However, it should always be possible to specify more parent
// nodes in the fragment, even if they are not within the tree being
// searched.
//
// To illustrate this, suppose you have a subtree that looks like this
//
//   E tr
//     E td
//     E td
//     E td
//     E td
//
// and you want to call Find to get the <td> elements. You cannot use
//
//   Find(subtree, `<td>`)
//
// but it is still OK to use
//
//   Find(subtree, `<table><tr><td>`)
//
// even though there is no <table> in subtree. The matcher will look
// look in subtree's parents.
package htmlnode // import "xi2.org/x/htmlnode"

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/net/html"
)

// Compare returns true if node n1 has the same Type, Data and
// Namespace fields as n2, and if the attributes of n2 are equal to or
// are a subset of the attributes of n1.
func Compare(n1, n2 *html.Node) bool {
	if n1 == nil || n2 == nil {
		return false
	}
	if n1.Type != n2.Type || n1.Data != n2.Data ||
		n1.Namespace != n2.Namespace {
		return false
	}
	am := map[html.Attribute]struct{}{}
	for _, a := range n1.Attr {
		am[a] = struct{}{}
	}
	for _, a := range n2.Attr {
		if _, ok := am[a]; !ok {
			return false
		}
	}
	return true
}

// Match compares the slice of nodes obtained by tracing n1's root
// node down to n1 with the equivalent slice obtained by tracing n2's
// root down to n2. Call these slices ns1 and ns2. If the tail of ns1
// matches ns2 with respect to Compare then Match returns true.
func Match(n1 *html.Node, n2 *html.Node) bool {
	for n1 != nil && n2 != nil {
		if !Compare(n1, n2) {
			return false
		}
		n1 = n1.Parent
		n2 = n2.Parent
	}
	if n1 == nil && n2 != nil {
		return false
	}
	return true
}

// Leaf converts an HTML fragment into a parse tree (without
// html/head/body ElementNodes or DoctypeNode), and then from the root
// of this tree repeatedly follows FirstChild until it finds a leaf
// node. This leaf node is returned as its result. In order to parse
// fragment, Leaf calls html.ParseFragment with a context of
// html.Node{Type: html.ElementNode}. If there is an error parsing
// fragment or no nodes are returned then Leaf returns a node
// of type html.ErrorNode. The return value of Leaf is intended to be
// passed to Match as its second argument.
func Leaf(fragment string) *html.Node {
	ns, err := html.ParseFragment(
		strings.NewReader(fragment), &html.Node{Type: html.ElementNode})
	if err != nil || len(ns) == 0 {
		return &html.Node{Type: html.ErrorNode}
	}
	n := ns[0]
	if n == nil {
		return nil
	}
	for n.FirstChild != nil {
		n = n.FirstChild
	}
	return n
}

// Attr returns the Val field of the first attribute in n.Attr whose
// Key field is equal to key. The second return value indicates if the
// node has such an attribute. If no such attribute exists Attr
// returns ("",false). Note that the Namespace fields of n.Attr are
// not compared.
func Attr(n *html.Node, key string) (string, bool) {
	if n == nil {
		return "", false
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

// AttrNS is like Attr but additionally compares the Namespace fields
// in the attributes.
func AttrNS(n *html.Node, namespace, key string) (string, bool) {
	if n == nil {
		return "", false
	}
	for _, a := range n.Attr {
		if a.Key == key && a.Namespace == namespace {
			return a.Val, true
		}
	}
	return "", false
}

// NextSibElt returns the next sibling of node n with type
// html.ElementNode (or nil if no such sibling).
func NextSibElt(n *html.Node) *html.Node {
	if n == nil {
		return nil
	}
	for n.NextSibling != nil {
		if n.NextSibling.Type == html.ElementNode {
			return n.NextSibling
		}
		n = n.NextSibling
	}
	return nil
}

// PrevSibElt behaves like NextSibElt, but returns the previous
// sibling html.ElementNode instead.
func PrevSibElt(n *html.Node) *html.Node {
	if n == nil {
		return nil
	}
	for n.PrevSibling != nil {
		if n.PrevSibling.Type == html.ElementNode {
			return n.PrevSibling
		}
		n = n.PrevSibling
	}
	return nil
}

// Next returns the next node in a depth first traversal of the tree
// at root (where the current node is node n), together with a delta
// indicating by how much it has descended or ascended the tree
// (descending being positive). When there are no more nodes it
// returns nil. If a value of nil is supplied for root it is assumed
// that the root node is the first node encountered with Parent ==
// nil.
func Next(n *html.Node, root *html.Node) (*html.Node, int) {
	delta := 0
	if n == nil {
		return nil, delta
	}
	if n.FirstChild != nil {
		delta += 1
		return n.FirstChild, delta
	}
	for n.NextSibling == nil {
		if n.Parent == nil || n == root {
			return nil, delta
		}
		n = n.Parent
		delta -= 1
	}
	if n == root {
		return nil, delta
	}
	return n.NextSibling, delta
}

// Prev behaves like Next, but returns the previous node instead.
func Prev(n *html.Node, root *html.Node) (*html.Node, int) {
	delta := 0
	if n == nil {
		return nil, delta
	}
	if n.LastChild != nil {
		delta += 1
		return n.LastChild, delta
	}
	for n.PrevSibling == nil {
		if n.Parent == nil || n == root {
			return nil, delta
		}
		n = n.Parent
		delta -= 1
	}
	if n == root {
		return nil, delta
	}
	return n.PrevSibling, delta
}

// Find is for locating nodes matching fragment within root. It first
// converts fragment into a leaf node (call it n2) using the Leaf
// function. It then does a depth first search of root and returns the
// slice of all nodes n in root which satisfy Match(n,n2). If there
// are no such nodes it returns the empty slice.
//
// Please note that fragment must parse in the context of having a
// generic element node as its parent, since it is passed to Leaf. See
// "A note on fragments" in the introduction for more details.
func Find(root *html.Node, fragment string) []*html.Node {
	var result []*html.Node
	n, n2 := root, Leaf(fragment)
	for n != nil {
		if Match(n, n2) {
			result = append(result, n)
		}
		n, _ = Next(n, root)
	}
	return result
}

// Flatten walks the tree under root finding all html.TextNodes and
// returns the string resulting from appending all their Data fields.
func Flatten(root *html.Node) string {
	var s string
	for n := root; n != nil; n, _ = Next(n, root) {
		if n.Type == html.TextNode {
			s += n.Data
		}
	}
	return s
}

// String returns a human readable representation of the single node
// n, with optional terminal colouring using ANSI escape codes. The
// representation begins with a capital letter indicating the
// html.NodeType. These are one of: X - ErrorNode, T - TextNode, R -
// DocumentNode, E - ElementNode, C - CommentNode, D - DoctypeNode.
func String(n *html.Node, colour bool) string {
	if n == nil {
		return ""
	}
	red, gre, yel := "\033[31m", "\033[32m", "\033[33m"
	blu, mag, cya := "\033[34m", "\033[35m", "\033[36m"
	rst := "\033[0m"
	c := func(str, col string) string {
		if colour {
			var cs string
			for _, s := range strings.Split(str, "\n") {
				cs = cs + col + s + rst + "\n"
			}
			return cs[:len(cs)-1]
		}
		return str
	}
	switch n.Type {
	case html.ErrorNode:
		return c("X ", mag) + c(n.Data, blu)
	case html.TextNode:
		return c("T ", mag) + n.Data
	case html.DocumentNode:
		return c("R ", mag) + c(n.Data, blu)
	case html.ElementNode:
		var attrs string
		for _, a := range n.Attr {
			name := c(a.Key, yel)
			sVal := fmt.Sprintf("%#v", a.Val)
			if a.Namespace != "" {
				name = c(a.Namespace, yel) + ":" + name
			}
			attrs += " " + name + "=" + c(sVal, cya)
		}
		name := c(n.Data, red)
		if n.Namespace != "" {
			name = c(n.Namespace, red) + ":" + name
		}
		return c("E ", mag) + name + attrs
	case html.CommentNode:
		return c("C ", mag) + c(n.Data, gre)
	case html.DoctypeNode:
		return c("D ", mag) + c(n.Data, blu)
	}
	return ""
}

// PrintTree prints the tree at root to the supplied io.Writer using
// String to print the nodes. It uses indention to convey the document
// structure. Like String, it can optionally colourize the output. It
// skips printing whitespace-only nodes of type html.TextNode.
//
// PrintTree returns any error it gets when calling fmt.Fprintf.
func PrintTree(w io.Writer, root *html.Node, colour bool) error {
	indent, n := "", root
	var delta int
	for n != nil {
		if n.Type != html.TextNode || strings.Trim(n.Data, "\r\n\t ") != "" {
			// print (skipping whitespace only TextNodes)
			_, err := fmt.Fprintf(w, "%s%s\n", indent, String(n, colour))
			if err != nil {
				return err
			}
		}
		n, delta = Next(n, root)
		if delta == 1 {
			indent += "  "
			continue
		}
		for delta < 0 {
			indent = indent[:len(indent)-2]
			delta++
		}
	}
	return nil
}

// Print calls PrintTree, using os.Stdout as the io.Writer and with
// colour set to true.
func Print(root *html.Node) error {
	return PrintTree(os.Stdout, root, true)
}
