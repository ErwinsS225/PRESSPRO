package billing

// ForcePlanForTest bascule un tenant sur un plan (tests uniquement).
func (s *Service) ForcePlanForTest(tenantID, planCode, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.subs[tenantID]
	if !ok {
		rec = &SubscriptionRecord{}
		s.subs[tenantID] = rec
	}
	rec.PlanCode = planCode
	rec.Status = status
	rec.TrialStartedAt = nil
	rec.TrialEndsAt = nil
	rec.PlanName = planCode
}
