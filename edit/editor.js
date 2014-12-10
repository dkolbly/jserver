
// this is evaluated at page load time...

var filename = 'test.html';

var editor = ace.edit("editor");
editor.setTheme("ace/theme/chrome");
editor.setFontSize(12);

var s = editor.getSession();
s.setMode("ace/mode/html");
var rhs = document.getElementById("page")
//rhs.srcdoc = s.getValue();

var modified = false;

function set_clean() {
    $("#formcomment").val("")
    $("#formcomment").prop('disabled', true);
    $("#pubbutton").prop('disabled', true);
    $("#pubinfo").text("");
    modified = false;
}

var edit_ready = false;

function set_dirty() {
    if (edit_ready) {
        $("#formcomment").val("A new version!")
        $("#formcomment").prop('disabled', false);
        $("#pubbutton").prop('disabled', false);
        $("#pubinfo").text("Page is modified");
        modified = true;
    }
}

s.on('change', function (e) {
    var pg = s.getValue();
    console.log("changed!: " + pg.substr(0,10))
    if (!modified) {
        set_dirty();
    }
    document.getElementById("page").srcdoc = pg;
})

$("#pagelink").attr("href", "/test.html")
$("#pagelink").text("test.html")

function loadother() {
    console.log("loading other...");
    $('#loadModal').modal('show')
}

function loadold() {
    console.log("loading old...");
}

function createnew() {
    console.log("new stuff")
    $('#newModal').modal('hide')
}


function publish() {
    console.log("publishing...");
    // see https://developer.mozilla.org/en-US/docs/Web/Guide/HTML/Forms/Sending_forms_through_JavaScript

    var XHR = new XMLHttpRequest();
    var FD = new FormData();
    
    var comment = document.getElementById("formcomment").value;
    var body = editor.getSession().getValue();
    console.log("filename = [" + filename + "]");
    console.log("comment = [" + comment + "]");

    FD.append("filename", filename)
    FD.append("comment", comment)
    FD.append("body", body)
    
    XHR.open('POST', '/edit/html')
    XHR.send(FD)
    set_clean();
}

function loadpage() {
    $.ajax({
        url: "/edit/v/" + filename
    }).then(function (data) {
        console.log("got the page...")
        data = $.parseJSON(data);
        editor.getSession().setValue(data.content);
        rhs.srcdoc = data.content;
        set_clean();
    })
}

function loadfiles() {
    $.ajax({
        url: "/edit/list"
    }).then(function (data) {
        data = $.parseJSON(data);
        console.log(data);
        $("#filelist").empty();

        $.each(data.listing,
               function (i, item) {
                   var but = $('<button class="btn-xs btn-default">').text("Open");
                   but.click(function () {
                       console.log("Click! <" + item.name + ">");
                       $('#loadModal').modal('hide');
                       filename = item.name;
                       $("#pagelink").attr("href", "/" + filename)
                       $("#pagelink").text(filename)
                       loadpage();
                       loadfiles();
                   })
                   var tr = $('<tr>').append(
                       $('<td>').text(item.name),
                       $('<td>').text(item.size),
                       $('<td>').text(item.modified),
                       $('<td>').append(but)
                   );
                   tr.appendTo('#filelist');
               });
    })
}

$(document).ready(function() {
    // this is invoked when the whole document is ready
    loadpage();
    loadfiles();

    var first_load = false
    $("#page").load(function () {
        if (!first_load) {
            first_load = true;
            edit_ready = true;
            document.getElementById("page").srcdoc = s.getValue();
            $("#page").css("opacity", "1");
            $("#page").fadeIn(1000);
        }
    })
})

