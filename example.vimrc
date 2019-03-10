" TODO
"
" * implement auto server startup with job_start
" * implement reconnect with backoff

call system("govim -debug")

call ch_logfile('/tmp/channellog', 'w')

func s:govimHandler(ch, m)
  " at this point the message will simply be our id
  let context = {'id': a:m}
  echom "Connected to govim as ".a:m
  func context.handle(channel, msg)
    echom self.id . " received message " . a:msg
  endfunc
  call ch_setoptions(a:ch, {'callback': context.handle})
endfunc

let channel = ch_open('localhost:1982', {"callback": function("s:govimHandler")})
