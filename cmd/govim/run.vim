if $GOVIMTEST_SOCKET != ""
  let channel = ch_open($GOVIMTEST_SOCKET)
else
  let job = job_start(["gobin", "-m", "-run", "github.com/myitcv/govim/cmd/govim"], {"in_mode": "json", "out_mode": "json", "err_mode": "json"})
  let channel = job_getchannel(job)
endif
call ch_sendexpr(channel, "hello")
call ch_sendexpr(channel, "hello")
