package main

import "secure-switch-backend/internal/wol"

func SendWol(h *Host) error { return wol.SendWol(h) }
