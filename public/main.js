var socket = io();

$('form').submit(function(e){
  e.preventDefault();
  e.stopPropagation()
  var msg = {
    user: $('#user').val(),
    text: $('#message').val(),
    timestamp: new Date().toISOString()
  }
  $('#message').val('');
  socket.emit('chat message', JSON.stringify(msg));
  return false;
});

var addMessage = function (msg) {
  var date = new Date(msg.timestamp).toLocaleString()
  var currentUser = $('#user').val() == msg.user
  console.log(currentUser)
  var tpl = `
    <li>
      <div class="timestamp">${date.substr(date.indexOf(', ') + 1)}</div>
      <div class="user ${currentUser ? 'current' : ''}">${msg.user}</div>
      <div class="message">${msg.text}</div>
    </li>
  `;
  $('#messages ul').append($(tpl))
  $('#messages')[0].scrollTop = 9999
}

socket.on('chat message', function(data){
  var msg = JSON.parse(data)
  addMessage(msg)
});

socket.on('chat history', function (all) {
  if (all) {
    all.split('\n').forEach(function (data) {
      var msg = JSON.parse(data)
      addMessage(msg)
    });
  }
})
