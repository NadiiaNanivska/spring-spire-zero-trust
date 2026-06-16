package internal

func (p *JVMAttestor) SetProcFSForTest(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.procFS = path
}
