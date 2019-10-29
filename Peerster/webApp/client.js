$(document).ready(function(){

	var guiport = "13081"
	var url = "http://127.0.0.1:" + guiport + "/"
	// Get id
	$.ajax({
		url: url + "id",
		type: "GET",
		dataType: "json",
		success: function(json) {

			console.log(json)
			var ID = json.id

			$("#NodeID").append("<P>" + ID)
		}
	})

	// Initialize buffer for msg and peernodes
	var peer_addrs = new Array();
	var rumors = new Array();

	// Define update peer_addrs and rumors func
	function updatePeers() {

		$.ajax({
			url: url + "node",
			type: "GET",
			dataType: "json",
			success: function(json){

				console.log(json)
				var new_peer_addrs = Array.from(json.nodes, x => x);

				// Check for updates
				if (new_peer_addrs.length > peer_addrs.length) {

					peer_addrs = new_peer_addrs;

					// Clear old display
					$("#PeerBox").empty();

					// Create new display
					peer_addrs.forEach((v, i) => {

						$("#PeerBox").append("<P>" + v);
					});
				}

 			}
 		});
	};

	function updateMsg() {

		$.ajax({
			url: url + "message",
			type: "GET",
			dataType: "json",
			success: function(json){

				var new_updated_rumors = Array.from(json.messages, x => x);
				console.log(new_updated_rumors)
				// Check for updates
				if (new_updated_rumors.length > rumors.length) {

					rumors = new_updated_rumors;

					$("#MsgBox").empty();

					rumors.forEach((v, i) => {

						$("#MsgBox").append("<P>Origin: " + v.Origin +  " ID: " + v.ID + " Text: " + v.Text);
					});
				}
 			}
 		});
	}

	// Run update periodically
	setInterval(updatePeers, 1000);
	setInterval(updateMsg, 1000);

	// Define handler for add msg
	$("#InputBtn").click(function(){

		// Get the text in textarea
		var text = $("#InputMsg").val();

		// Refresh textarea
		$("#InputMsg").val("");

		// Send text to server
		var data = {text : text};

		$.ajax({
			url: url + "message",
			type: "POST",
			data: JSON.stringify(data),
			dataType: "json",
			success: function(msg) {

				alert("Successfully add msg!!!")
			}
		});
	});

	// Define handler for add pere
	$("#PeerAddBtn").click(function(){

		// Get the text in the textarea
		var text = $("#PeerAddr").val();

		// Refresh peer addr
		$("#PeerAddr").val("");

		var regExp = RegExp("((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)");
		console.log("New peer adding feature!!");
		console.log(regExp.test(text));
		if (!regExp.test(text)) {

			alert("Bad IP address!!!");
			return;
		}
		// Send new addr to server
		var data = {addr : text};

		$.ajax({
			url: url + "node",
			type: "POST",
			data: JSON.stringify(data),
			dataType: "json",
			success: function(msg) {

				alert("Successfully add peer");
			}
		});
	});
})

