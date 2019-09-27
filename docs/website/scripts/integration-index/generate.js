const fs = require('fs');
const readYaml = require('read-yaml');
const Set = require("collections/set");

const config = readYaml.sync('integrations.yaml');
const out = "integrations.md"
const stream = fs.createWriteStream(out);  //, {flags:'a'});



function all() {
    writeHeader()
    writeIndex()
    writeTools()
    writeIntegrations()
}
function writeIntegrations() {
    writeln("## Integration Details")
    Object.keys(config.integrations).forEach(function(key) { writeIntegration(key, config.integrations[key]) })
}
function writeIntegration(id, integ) {
    writeln('')
    write('###  ')
    writeAnchor(integ.title, id)
    writeln('')
    writeln(integ.description)
    if ('software' in integ) {
        write("\n**Software**: ")
        for (var i = 0; i < integ.software.length; i++) {
            if (i > 0) {
                write(', ')
            }
            writeSoftware(integ.software[i])
        }
        writeln('')
    }
    if ('inventors' in integ) {
        write("\n**Inventors**: ")
        for (var i = 0; i < integ.inventors.length; i++) {
            if (i > 0) {
                write(', ')
            }
            writeInventor(integ.inventors[i])
        }
        writeln('')
    }
    if ('tutorials' in integ) {
        writeln("\n**Tutorials**")
        integ.tutorials.forEach(function(x) { write("* "); writeTutorial(x); writeln('') })
    }
    if ('code' in integ) {
        writeln("\n**Code**")
        integ.code.forEach(function(x) { write("* "); writeCode(x); writeln('') })
    }
    if ('videos' in integ) {
        writeln("\n**Videos**")
        integ.videos.forEach(function(x) { write("* "); writeVideo(x); writeln('') })
    }
    if ('blogs' in integ) {
        writeln("\n**Blogs**")
        integ.blogs.forEach(function(x) { write("* "); writeBlog(x); writeln('') })
    }
}

function writeSoftware(soft) {
    if (!(soft in config.software)) {
        write(soft)
    } else {
        name = config.software[soft].name
        link = config.software[soft].link
        write('<a href="' + link + '">' + name + '</a>')
    }
}

function writeInventor(x) {
    if (typeof x == 'object') {
        write(x.name)
        if ('organization' in x) {
            write(' at ')
            writeOrganization(x.organization)           
        }
    } else {
        writeOrganization(x)
    } 
}

function writeOrganization(org) {
    if (org in config.organizations) {
        name = config.organizations[org].name
        link = config.organizations[org].link
        write('<a href="' + link + '">' + name + '</a>')
    } else {
        write(org)
    }
}

function writeAnchor(text, name) {
    write('<a name="' + name + '">' + text + '</a>')
}
function writeLink(text, url) {
    write('<a href="' + url + '">' + text + '</a>')
}
function writeVideo(video) {
    if (typeof(video) != "object") {
        writeLink(video, video)
    } else {
        write('<a href="' + video.link + '">' 
                + video.title 
                + '</a> by ') 
        writeVideoSpeakers(video.speakers) 
        write(' at ' 
                + video.venue)
    }
}

function writeVideoSpeakers(speakers) {
    for (var i=0; i < speakers.length; i++) {
        if (i > 0) {
            write(", ")
        }
        write(speakers[i].name)
        if (speakers[i].organization in config.organizations) {
            org = config.organizations[speakers[i].organization]
            write(' from ')
            write('<a href="' + org.link + '">')
            write(org.name)
            write('</a>')
        } else {
            write(' from ')
            write(speakers[i].organization)
        }
    }
}

function writeBlog(x) {
    writeLink(x, x)
}

function writeTutorial(tutorial) {
    writeLink(tutorial, tutorial)
}

function writeCode(code) {
    writeLink(code, code)
}

function writeIndex() {
    var layers = new Set();
    writeln("## Index of Enforcement Points by Layer of the Stack")
    Object.keys(enforcements).forEach(function(key) { 
        c = enforcements[key]
        if ('labels' in c && 'layer' in c.labels) {
            layers.add(c.labels.layer)
        } else {
            layers.add(undefined)
        }
    })
    layers.forEach(writeIndexLayer)

}

function writeIndexLayer(layer) {
    if (layer == undefined) {
        writeln('\n**unspecified**')
    } else {
        writeln("\n**" + layer + "**")
    }
    // sort by title name
    keys = Object.keys(config.integrations)
    objs = []
    for (var i = 0; i < keys.length; i++) {
        objs.push({key: keys[i], obj: config.integrations[keys[i]]})
    }
    objs.sort((a,b) =>  a.obj.title.localeCompare(b.obj.title))  
    objs.forEach(function (x) {
        key = x.key
        integ = x.obj
        if (getLayer(integ) != layer) {
            return
        }
        write("* ")
        writeLink(integ.title, "#" + key)
        writeln('')
    })
}

function getLayer(integ) {
    try {
        return integ.labels.layer
    } catch (e) {
        return undefined
    }
}
function findTools() {
    d = {}
    Object.keys(config.integrations).forEach(function(key) { 
        c = config.integrations[key]
        if ('labels' in c && 'type' in c.labels && c.labels.type == 'poweredbyopa') {
            d[key] = c
        }
    })  
    return d
}

function findEnforcements() {
    d = {}
    Object.keys(config.integrations).forEach(function(key) { 
        c = config.integrations[key]
        if ('labels' in c && 'layer' in c.labels && (!('type' in c.labels) || c.labels.type == 'enforcement')) {
            d[key] = c
        }
    })  
    return d
}

function writeTools() {
    var layers = new Set();
    writeln("## Index of Tools Powered by OPA")
    keys = Object.keys(tools)
    for (var i = 0; i < keys.length; i++) {
        if (i > 0) {
            write(", ")
        }
        writeTool(keys[i])
    }
}

function writeTool(id) {
    write("* ")
    writeLink(tools[id].title, "#" + key)
    writeln('')
}

function writeHeader() {
    stream.write("# Integration Index (Total " + Object.keys(config.integrations).length + ")\n")
}

function writeln(string) {
    stream.write(string + '\n')
}
function write(string) {
    stream.write(string)
}


tools = findTools()
enforcements = findEnforcements()
all()

