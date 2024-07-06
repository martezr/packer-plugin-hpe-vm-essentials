package common

/*
// waitForState simply blocks until the server is in a state we expect,
// while eventually timing out.
func waitForServerState(serverID int64, client *morpheus.Client, timeout time.Duration) error {
	done := make(chan struct{})
	defer close(done)
	result := make(chan error, 1)
	go func() {
		attempts := 0
		for {
			attempts++
			log.Printf("Checking server status... (attempt: %d)", attempts)
			serverInfo, err := client.GetInstance(serverID, &morpheus.Request{})
			if err != nil {
				result <- err
				return
			}

			if serverInfo.Status == state && (serverInfo.PowerStatus == power || power == "") {
				result <- nil
				return
			}

			time.Sleep(3 * time.Second)

			// Verify we shouldn't exit
			select {
			case <-done:
				// We finished, so just exit the goroutine
				return
			default:
				// Keep going
			}
		}
	}()
	log.Printf("Waiting for up to %d seconds for server", timeout/time.Second)
	select {
	case err := <-result:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timeout while waiting for server")
	}
}
*/
