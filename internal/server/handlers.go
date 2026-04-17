package server

import "net/http"

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request)  {}
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {}
func (s *Server) handleTags(w http.ResponseWriter, r *http.Request)   {}
func (s *Server) handleNote(w http.ResponseWriter, r *http.Request)   {}
